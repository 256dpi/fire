package blaze

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// Bucket provides file storage capabilities.
type Bucket struct {
	store    *coal.Store
	notary   *heat.Notary
	service  Service
	register *Register
}

// NewBucket creates a new bucket.
func NewBucket(store *coal.Store, notary *heat.Notary, service Service, register *Register) *Bucket {
	return &Bucket{
		store:    store,
		notary:   notary,
		service:  service,
		register: register,
	}
}

// Upload will initiate and perform an upload using the provided callback and
// return a claim key and the uploaded file. Upload must be called outside a
// transaction to ensure the uploaded file is tracked in case of errors.
func (s *Bucket) Upload(ctx context.Context, name, mediaType string, cb func(Upload) (int64, error)) (string, *File, error) {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.Upload")
	span.Tag("type", mediaType)
	defer span.End()

	// check transaction
	if coal.HasTransaction(ctx) {
		return "", nil, xo.F("unexpected transaction for upload")
	}

	// check name
	if len(name) > maxFileNameLength {
		return "", nil, xo.SF("file name too long")
	}

	// set default type
	if mediaType == "" {
		if name != "" {
			mediaType = serve.MimeTypeByExtension(path.Ext(name), false)
		}
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
	}

	// create handle
	handle, err := s.service.Prepare(ctx)
	if err != nil {
		return "", nil, xo.W(err)
	}

	// prepare file
	file := &File{
		Base:    coal.B(),
		State:   Uploading,
		Updated: time.Now(),
		Name:    name,
		Type:    mediaType,
		Handle:  handle,
	}

	// validate file
	err = file.Validate()
	if err != nil {
		return "", nil, err
	}

	// create file
	err = s.store.M(file).Insert(ctx, file)
	if err != nil {
		return "", nil, err
	}

	// begin upload
	upload, err := s.service.Upload(ctx, handle, name, mediaType)
	if err != nil {
		return "", nil, xo.W(err)
	}

	// perform upload
	size, err := cb(upload)
	if err != nil {
		return "", nil, xo.W(err)
	}

	// get time
	now := time.Now()

	// set fields
	file.State = Uploaded
	file.Updated = now
	file.Size = size

	// validate file
	err = file.Validate()
	if err != nil {
		return "", nil, err
	}

	// update file
	_, err = s.store.M(file).Update(ctx, nil, file.ID(), bson.M{
		"$set": bson.M{
			"State":   Uploaded,
			"Updated": now,
			"Size":    size,
		},
	}, false)
	if err != nil {
		return "", nil, err
	}

	// issue claim key
	claimKey, err := s.notary.Issue(&ClaimKey{
		File: file.ID(),
		Size: file.Size,
		Name: file.Name,
		Type: file.Type,
	})
	if err != nil {
		return "", nil, err
	}

	return claimKey, file, nil
}

// UploadAction returns an action that provides an upload endpoint that stores
// files and returns claim keys. The action should be protected and only allow
// authorized clients.
func (s *Bucket) UploadAction(limit int64) *fire.Action {
	// set default limit
	if limit == 0 {
		limit = serve.MustByteSize("8M")
	}

	return fire.A("blaze/Bucket.UploadAction", []string{"POST"}, limit, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return xo.F("stores must be identical")
		}

		// get raw content type
		rawContentType := ctx.HTTPRequest.Header.Get("Content-Type")

		// get content type
		contentType, ctParams, err := mime.ParseMediaType(rawContentType)
		if rawContentType != "" && err != nil {
			ctx.ResponseWriter.WriteHeader(http.StatusBadRequest)
			return nil
		}

		// get content length
		contentLength := ctx.HTTPRequest.ContentLength
		if contentLength != -1 && contentLength > limit {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		}

		// upload multipart or raw
		var keys []string
		if contentType == "multipart/form-data" {
			keys, err = s.uploadMultipart(ctx, ctParams["boundary"])
		} else {
			keys, err = s.uploadBody(ctx, contentType)
		}

		// check limit error
		if err != nil && strings.HasSuffix(err.Error(), serve.ErrBodyLimitExceeded.Error()) {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		} else if err != nil {
			return err
		}

		// respond with keys
		return ctx.Respond(stick.Map{
			"keys": keys,
		})
	})
}

func (s *Bucket) uploadBody(ctx *fire.Context, mediaType string) ([]string, error) {
	// prepare filename
	filename := ""

	// parse content disposition
	if ctx.HTTPRequest.Header.Get("Content-Disposition") != "" {
		disposition, params, err := mime.ParseMediaType(ctx.HTTPRequest.Header.Get("Content-Disposition"))
		if err != nil {
			return nil, err
		}

		// check disposition
		if disposition != "" && disposition != "attachment" {
			return nil, xo.SF("expected attachment content disposition")
		}

		// get filename
		filename = params["filename"]
	}

	// upload stream
	claimKey, _, err := s.Upload(ctx, filename, mediaType, func(upload Upload) (int64, error) {
		return UploadFrom(upload, ctx.HTTPRequest.Body)
	})
	if err != nil {
		return nil, err
	}

	return []string{claimKey}, nil
}

func (s *Bucket) uploadMultipart(ctx *fire.Context, boundary string) ([]string, error) {
	// prepare reader
	reader := multipart.NewReader(ctx.HTTPRequest.Body, boundary)

	// get first part
	part, err := reader.NextPart()
	if err != nil && err != io.EOF {
		return nil, xo.W(err)
	}

	// collect claim keys
	var claimKeys []string

	// handle all parts
	for part != nil {
		// parse content type
		contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return nil, xo.W(err)
		}

		// upload part
		claimKey, _, err := s.Upload(ctx, part.FileName(), contentType, func(upload Upload) (int64, error) {
			return UploadFrom(upload, part)
		})
		if err != nil {
			return nil, err
		}

		// add claim key
		claimKeys = append(claimKeys, claimKey)

		// get next part
		part, err = reader.NextPart()
		if err != nil && err != io.EOF {
			return nil, xo.W(err)
		}
	}

	return claimKeys, nil
}

// Claim will claim the link at the field on the provided model. The claimed
// link must be persisted in the same transaction as the claim to ensure
// consistency.
func (s *Bucket) Claim(ctx context.Context, model coal.Model, field string) error {
	// get value
	value := stick.MustGet(model, field)

	// lookup binding
	binding := s.register.Lookup(model, field)
	if binding == nil {
		return xo.F("missing binding")
	}

	// get link
	var link *Link
	switch value := value.(type) {
	case Link:
		link = &value
	case *Link:
		link = value
	}

	// check link
	if link == nil {
		return xo.F("missing link")
	}

	// claim file
	err := s.ClaimLink(ctx, link, binding.Name, model.ID())
	if err != nil {
		return err
	}

	// set link
	switch value.(type) {
	case Link:
		stick.MustSet(model, field, *link)
	case *Link:
		stick.MustSet(model, field, link)
	}

	return nil
}

// ClaimLink will claim the provided link under the specified binding. The
// claimed link must be persisted in the same transaction as the claim to ensure
// consistency.
func (s *Bucket) ClaimLink(ctx context.Context, link *Link, binding string, owner coal.ID) error {
	// check transaction
	if !coal.HasTransaction(ctx) {
		return xo.F("missing transaction for claim")
	}

	// check file
	if !link.File.IsZero() {
		return xo.F("existing claimed filed")
	}

	// check claim key
	if link.ClaimKey == "" {
		return xo.F("missing claim key")
	}

	// claim file
	file, err := s.ClaimFile(ctx, link.ClaimKey, binding, owner)
	if err != nil {
		return err
	}

	// update link
	*link = Link{
		Ref:      link.Ref,
		File:     file.ID(),
		FileName: file.Name,
		FileType: file.Type,
		FileSize: file.Size,
	}

	return nil
}

// ClaimFile will claim the file referenced by the provided claim key using the
// specified binding and owner.
func (s *Bucket) ClaimFile(ctx context.Context, claimKey, binding string, owner coal.ID) (*File, error) {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.ClaimFile")
	defer span.End()

	// get binding
	bnd := s.register.Get(binding)
	if bnd == nil {
		return nil, xo.F("unknown binding: %s", binding)
	}

	// check owner
	if owner.IsZero() {
		return nil, xo.F("missing owner")
	}

	// verify claim key
	var key ClaimKey
	err := s.notary.Verify(&key, claimKey)
	if err != nil {
		return nil, err
	}

	// verify limit
	if bnd.Limit > 0 && key.Size > bnd.Limit {
		return nil, xo.F("too big")
	}

	// verify type
	if len(bnd.Types) > 0 && !stick.Contains(bnd.Types, key.Type) {
		return nil, xo.F("unsupported type: %s", key.Type)
	}

	// claim file
	var file File
	found, err := s.store.M(&File{}).UpdateFirst(ctx, &file, bson.M{
		"_id":   key.File,
		"State": Uploaded,
	}, bson.M{
		"$set": bson.M{
			"State":   Claimed,
			"Updated": time.Now(),
			"Binding": binding,
			"Owner":   owner,
		},
	}, nil, false)
	if err != nil {
		return nil, err
	} else if !found {
		return nil, xo.F("unable to claim file")
	}

	return &file, nil
}

// Release will release the link at the field on the provided model. The
// released link must be persisted in the same transaction as the release to
// ensure consistency.
func (s *Bucket) Release(ctx context.Context, model coal.Model, field string) error {
	// get field
	value := stick.MustGet(model, field)

	// get link
	var link *Link
	switch value := value.(type) {
	case Link:
		link = &value
	case *Link:
		link = value
	}

	// check link
	if link == nil {
		return xo.F("missing link")
	}

	// release link
	err := s.ReleaseLink(ctx, link)
	if err != nil {
		return err
	}

	// unset link
	switch value.(type) {
	case Link:
		stick.MustSet(model, field, Link{})
	case *Link:
		stick.MustSet(model, field, nil)
	}

	return nil
}

// ReleaseLink will release the provided link. The released link must be
// persisted in the same transaction as the release to ensure consistency.
func (s *Bucket) ReleaseLink(ctx context.Context, link *Link) error {
	// get file
	file := link.File
	if file.IsZero() {
		return xo.F("invalid file id")
	}

	// check transaction
	if !coal.HasTransaction(ctx) {
		return xo.F("missing transaction for release")
	}

	// release file
	err := s.ReleaseFile(ctx, file)
	if err != nil {
		return err
	}

	return nil
}

// ReleaseFile will release the file with the provided id.
func (s *Bucket) ReleaseFile(ctx context.Context, file coal.ID) error {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.ReleaseFile")
	defer span.End()

	// release file
	found, err := s.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
		"_id":   file,
		"State": Claimed,
	}, bson.M{
		"$set": bson.M{
			"State":   Released,
			"Updated": time.Now(),
			"Binding": "",
			"Owner":   nil,
		},
	}, nil, false)
	if err != nil {
		return err
	} else if !found {
		return xo.F("unable to release file")
	}

	return nil
}

// Modifier will handle modifications on all or just the specified link fields.
func (s *Bucket) Modifier(fields ...string) *fire.Callback {
	return fire.C("blaze/Bucket.Modifier", fire.Modifier, fire.Only(fire.Create|fire.Update|fire.Delete), func(ctx *fire.Context) error {
		// check store
		if ctx.Store != s.store {
			return xo.F("stores must be identical")
		}

		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// get owner
		owner := ctx.Model.ID()

		// check all fields
		for _, field := range fields {
			// get value
			value := stick.MustGet(ctx.Model, field)

			// get old value
			var oldValue interface{}
			if ctx.Original != nil {
				oldValue = stick.MustGet(ctx.Original, field)
			}

			// get binding
			binding := s.register.Lookup(ctx.Model, field)
			if binding == nil {
				return xo.F("missing binding")
			}

			// inspect type
			switch value := value.(type) {
			case Link:
				// get old link
				var oldLink *Link
				if oldValue != nil {
					l := oldValue.(Link)
					oldLink = &l
				}

				// get new link
				newLink := &value

				// swap on delete
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}

				// modify link
				err := s.modifyLink(ctx, newLink, oldLink, binding.Name, owner)
				if err != nil {
					return xo.WF(err, field)
				}

				// update link
				stick.MustSet(ctx.Model, field, value)
			case *Link:
				// get old link
				var oldLink *Link
				if oldValue != nil {
					oldLink = oldValue.(*Link)
				}

				// get new link
				newLink := value

				// swap on delete
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}

				// modify link
				err := s.modifyLink(ctx, newLink, oldLink, binding.Name, owner)
				if err != nil {
					return xo.WF(err, field)
				}

				// update link
				stick.MustSet(ctx.Model, field, value)
			case Links:
				// get old links
				var oldLinks Links
				if oldValue != nil {
					oldLinks = oldValue.(Links)
				}

				// get new links
				newLinks := value
				if ctx.Operation == fire.Delete {
					oldLinks = newLinks
					newLinks = nil
				}

				// modify links
				err := s.modifyLinks(ctx, newLinks, oldLinks, binding.Name, owner)
				if err != nil {
					return xo.WF(err, field)
				}

				// update links
				stick.MustSet(ctx.Model, field, value)
			default:
				return xo.F("%s: unsupported type: %T", field, value)
			}
		}

		return nil
	})
}

func (s *Bucket) modifyLink(ctx context.Context, newLink, oldLink *Link, binding string, owner coal.ID) error {
	// detect change
	added := oldLink == nil && newLink != nil
	updated := oldLink != nil && newLink != nil && newLink.ClaimKey != ""
	deleted := oldLink != nil && newLink == nil

	// release old file
	if updated || deleted {
		err := s.ReleaseLink(ctx, oldLink)
		if err != nil {
			return err
		}
	}

	// claim new file
	if added || updated {
		// unset file
		newLink.File = coal.ID{}

		// claim
		err := s.ClaimLink(ctx, newLink, binding, owner)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Bucket) modifyLinks(ctx context.Context, newLinks, oldLinks Links, binding string, owner coal.ID) error {
	// build map of new links
	newMap := make(map[string]*Link, len(newLinks))
	newRefs := make([]string, 0, len(newLinks))
	for i, link := range newLinks {
		if link.Ref == "" {
			return xo.F("missing reference")
		}
		newMap[link.Ref] = &newLinks[i]
		newRefs = append(newRefs, link.Ref)
	}

	// build map of old links
	oldMap := make(map[string]*Link, len(oldLinks))
	oldRefs := make([]string, 0, len(oldLinks))
	for i, link := range oldLinks {
		if link.Ref == "" {
			return xo.F("missing reference")
		}
		oldMap[link.Ref] = &oldLinks[i]
		oldRefs = append(oldRefs, link.Ref)
	}

	// determine added, kept and deleted links
	added := stick.Subtract(newRefs, oldRefs)
	kept := stick.Intersect(newRefs, oldRefs)
	deleted := stick.Subtract(oldRefs, newRefs)

	// determine updated links
	updated := make([]string, 0, len(kept))
	for _, ref := range kept {
		if newMap[ref].ClaimKey != "" {
			updated = append(updated, ref)
		}
	}

	// release old files
	for _, ref := range stick.Union(updated, deleted) {
		err := s.ReleaseLink(ctx, oldMap[ref])
		if err != nil {
			return err
		}
	}

	// claim new files
	for _, ref := range stick.Union(added, updated) {
		// get link
		link := newMap[ref]

		// unset file
		link.File = coal.ID{}

		// claim
		err := s.ClaimLink(ctx, link, binding, owner)
		if err != nil {
			return err
		}
	}

	return nil
}

// Decorate will populate the provided link if a file is available.
func (s *Bucket) Decorate(link *Link) error {
	// skip if file is missing
	if link == nil || link.File.IsZero() {
		return nil
	}

	// issue view key
	viewKey, err := s.notary.Issue(&ViewKey{
		File: link.File,
	})
	if err != nil {
		return err
	}

	// set key
	link.ViewKey = viewKey

	// copy info
	link.Name = link.FileName
	link.Type = link.FileType
	link.Size = link.FileSize

	return nil
}

// Decorator will populate all or just the specified link fields.
func (s *Bucket) Decorator(fields ...string) *fire.Callback {
	return fire.C("blaze/Bucket.Decorator", fire.Decorator, fire.All(), func(ctx *fire.Context) error {
		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// decorate model
		if ctx.Model != nil {
			err := s.decorateModel(ctx.Model, fields)
			if err != nil {
				return err
			}
		}

		// decorate models
		for _, model := range ctx.Models {
			err := s.decorateModel(model, fields)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Bucket) decorateModel(model coal.Model, fields []string) error {
	// collect fields if empty
	if len(fields) == 0 {
		fields = collectFields(model)
	}

	// iterate over all fields
	for _, field := range fields {
		// get value
		value := stick.MustGet(model, field)

		// inspect type
		var err error
		switch value := value.(type) {
		case Link:
			// decorate link
			err = s.Decorate(&value)
			if err != nil {
				return err
			}

			// update link
			stick.MustSet(model, field, value)
		case *Link:
			// decorate link
			err = s.Decorate(value)
			if err != nil {
				return err
			}
		case Links:
			// decorate links
			for i := range value {
				err = s.Decorate(&value[i])
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Download will initiate a download for the file referenced by the provided
// view key.
func (s *Bucket) Download(ctx context.Context, viewKey string) (Download, *File, error) {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.Download")
	defer span.End()

	// verify key
	var key ViewKey
	err := s.notary.Verify(&key, viewKey)
	if err != nil {
		return nil, nil, err
	}

	// download file
	download, file, err := s.DownloadFile(ctx, key.File)
	if err != nil {
		return nil, nil, err
	}

	return download, file, nil
}

// DownloadFile will initiate a download for the specified file.
func (s *Bucket) DownloadFile(ctx context.Context, id coal.ID) (Download, *File, error) {
	// find file
	var file File
	found, err := s.store.M(&File{}).FindFirst(ctx, &file, bson.M{
		"_id":   id,
		"State": Claimed,
	}, nil, 0, false)
	if err != nil {
		return nil, nil, err
	} else if !found {
		return nil, nil, xo.F("missing file")
	}

	// begin download
	download, err := s.service.Download(ctx, file.Handle)
	if err != nil {
		return nil, nil, xo.W(err)
	}

	return download, &file, nil
}

// DownloadAction returns an endpoint that allows downloading files using view
// keys. This action is usually publicly accessible.
func (s *Bucket) DownloadAction() *fire.Action {
	return fire.A("blaze/Bucket.DownloadAction", []string{"HEAD", "GET"}, 0, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return xo.F("stores must be identical")
		}

		// get key
		key := ctx.HTTPRequest.URL.Query().Get("key")
		if key == "" {
			ctx.ResponseWriter.WriteHeader(http.StatusBadRequest)
			return nil
		}

		// get dl
		dl := ctx.HTTPRequest.URL.Query().Get("dl") == "1"

		// initiate download
		download, file, err := s.Download(ctx, key)
		if err != nil {
			return err
		}

		// get binding
		binding := s.register.Get(file.Binding)
		if binding == nil {
			return xo.F("missing binding")
		}

		// prepare content disposition
		contentDisposition := "inline"
		if dl {
			// get filename
			filename := file.Name
			if binding.Filename != "" {
				filename = binding.Filename
			} else if filename == "" {
				filename = file.ID().Hex()
			}

			// set disposition
			contentDisposition = fmt.Sprintf(`attachment; filename="%s"`, filename)
		}

		// set content type, length and disposition
		ctx.ResponseWriter.Header().Set("Content-Type", file.Type)
		ctx.ResponseWriter.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
		ctx.ResponseWriter.Header().Set("Content-Disposition", contentDisposition)

		// unset any content security policy
		ctx.ResponseWriter.Header().Del("Content-Security-Policy")

		// stream download
		http.ServeContent(ctx.ResponseWriter, ctx.HTTPRequest, "", file.Updated, download)

		return nil
	})
}

// TODO: Periodically verify claimed files.

// CleanupTask will return a periodic task that can be run to periodically
// cleanup obsolete files.
func (s *Bucket) CleanupTask(lifetime, timeout, periodicity, retention time.Duration) *axe.Task {
	return &axe.Task{
		Job: &CleanupJob{},
		Handler: func(ctx *axe.Context) error {
			return s.Cleanup(ctx, retention)
		},
		Workers:     1,
		MaxAttempts: 1,
		Lifetime:    lifetime,
		Timeout:     timeout,
		Periodicity: periodicity,
		PeriodicJob: axe.Blueprint{
			Job: &CleanupJob{
				Base: axe.B("periodic"),
			},
		},
	}
}

// Cleanup will remove obsolete files and remove their blobs. Files in the
// states "uploading" or "uploaded" are removed after the specified retention
// which defaults to one hour if zero. Files in the states "released" and
// "deleting" are removed immediately. It will also allow the service to clean up.
func (s *Bucket) Cleanup(ctx context.Context, retention time.Duration) error {
	// set default retention
	if retention == 0 {
		retention = time.Hour
	}

	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.Cleanup")
	span.Tag("retention", retention.String())
	defer span.End()

	// get iterator for deletable files
	iter, err := s.store.M(&File{}).FindEach(ctx, bson.M{
		"$or": []bson.M{
			{
				"State": bson.M{
					"$in": bson.A{Uploading, Uploaded},
				},
				"Updated": bson.M{
					"$lt": time.Now().Add(-retention),
				},
			},
			{
				"State": bson.M{
					"$in": bson.A{Released, Deleting},
				},
			},
		},
	}, nil, 0, 0, false, coal.NoTransaction)
	if err != nil {
		return err
	}

	// iterate over files
	defer iter.Close()
	for iter.Next() {
		// decode file
		var file File
		err := iter.Decode(&file)
		if err != nil {
			return err
		}

		// flag file as deleting if not already
		if file.State != Deleting {
			found, err := s.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
				"_id":   file.ID(),
				"State": file.State,
			}, bson.M{
				"$set": bson.M{
					"State":   Deleting,
					"Updated": time.Now(),
				},
			}, nil, false)
			if err != nil {
				return err
			} else if !found {
				return nil
			}
		}

		// delete blob
		err = s.service.Delete(ctx, file.Handle)
		if err != nil {
			return xo.W(err)
		}

		// delete file
		_, err = s.store.M(&File{}).Delete(ctx, nil, file.ID())
		if err != nil {
			return err
		}
	}

	// check error
	err = iter.Error()
	if err != nil {
		return err
	}

	// cleanup service
	err = s.service.Cleanup(ctx)
	if err != nil {
		return xo.W(err)
	}

	return nil
}
