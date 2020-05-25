package blaze

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/256dpi/serve"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// Storage provides file storage services.
type Storage struct {
	store    *coal.Store
	notary   *heat.Notary
	service  Service
	register *Register
}

// NewStorage creates a new storage.
func NewStorage(store *coal.Store, notary *heat.Notary, service Service, register *Register) *Storage {
	return &Storage{
		store:    store,
		notary:   notary,
		service:  service,
		register: register,
	}
}

// Upload will initiate and perform an upload using the provided callback and
// return a claim key and the uploaded file. Upload must be called outside of a
// transaction to ensure the uploaded file is tracked in case of errors.
func (s *Storage) Upload(ctx context.Context, mediaType string, cb func(Upload) (int64, error)) (string, *File, error) {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.Upload")
	span.Log("type", mediaType)
	defer span.Finish()

	// check transaction
	if coal.HasTransaction(ctx) {
		return "", nil, fmt.Errorf("unexpected transaction for upload")
	}

	// set default type
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	// create handle
	handle, err := s.service.Prepare(ctx)
	if err != nil {
		return "", nil, err
	}

	// prepare file
	file := &File{
		Base:    coal.B(),
		State:   Uploading,
		Updated: time.Now(),
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
	upload, err := s.service.Upload(ctx, handle, mediaType)
	if err != nil {
		return "", nil, err
	}

	// perform upload
	size, err := cb(upload)
	if err != nil {
		return "", nil, err
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
		Type: file.Type,
	})
	if err != nil {
		return "", nil, err
	}

	return claimKey, file, nil
}

// UploadAction returns an action that provides an upload service that stores
// files and returns claim keys. The action should be protected and only allow
// authorized clients.
func (s *Storage) UploadAction(limit int64) *fire.Action {
	// set default limit
	if limit == 0 {
		limit = serve.MustByteSize("8M")
	}

	return fire.A("blaze/Storage.UploadAction", []string{"POST"}, limit, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
		}

		// get content type
		contentType, ctParams, err := mime.ParseMediaType(ctx.HTTPRequest.Header.Get("Content-Type"))
		if err != nil {
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

func (s *Storage) uploadBody(ctx *fire.Context, mediaType string) ([]string, error) {
	// upload stream
	claimKey, _, err := s.Upload(ctx, mediaType, func(upload Upload) (int64, error) {
		return UploadFrom(upload, ctx.HTTPRequest.Body)
	})
	if err != nil {
		return nil, err
	}

	return []string{claimKey}, nil
}

func (s *Storage) uploadMultipart(ctx *fire.Context, boundary string) ([]string, error) {
	// prepare reader
	reader := multipart.NewReader(ctx.HTTPRequest.Body, boundary)

	// get first part
	part, err := reader.NextPart()
	if err != nil && err != io.EOF {
		return nil, err
	}

	// collect claim keys
	var claimKeys []string

	// handle all parts
	for part != nil {
		// parse content type
		contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}

		// upload part
		claimKey, _, err := s.Upload(ctx, contentType, func(upload Upload) (int64, error) {
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
			return nil, err
		}
	}

	return claimKeys, nil
}

// Claim will claim the link at the field on the provided model. The claimed
// link must be persisted in the same transaction as the claim to ensure
// consistency.
func (s *Storage) Claim(ctx context.Context, model coal.Model, field string) error {
	// get value
	value := stick.MustGet(model, field)

	// lookup binding
	binding := s.register.Lookup(model, field)
	if binding == nil {
		return fmt.Errorf("missing binding")
	}

	// get link
	var link Link
	switch value := value.(type) {
	case Link:
		link = value
	case *Link:
		link = *value
	}

	// claim file
	err := s.ClaimLink(ctx, &link, binding.Name, model.ID())
	if err != nil {
		return err
	}

	// set link
	switch value.(type) {
	case Link:
		stick.MustSet(model, field, link)
	case *Link:
		stick.MustSet(model, field, &link)
	}

	return nil
}

// ClaimLink will claim the provided link under the specified binding. The
// claimed link must be persisted in the same transaction as the claim to ensure
// consistency.
func (s *Storage) ClaimLink(ctx context.Context, link *Link, binding string, owner coal.ID) error {
	// check transaction
	if !coal.HasTransaction(ctx) {
		return fmt.Errorf("missing transaction for claim")
	}

	// check file
	if link.File != nil {
		return fmt.Errorf("existing claimed filed")
	}

	// check claim key
	if link.ClaimKey == "" {
		return fmt.Errorf("missing claim key")
	}

	// claim file
	file, err := s.ClaimFile(ctx, link.ClaimKey, binding, owner)
	if err != nil {
		return err
	}

	// update link
	*link = Link{
		File:     coal.P(file.ID()),
		FileType: file.Type,
		FileSize: file.Size,
	}

	return nil
}

// ClaimFile will claim the file referenced by the provided claim key using the
// specified binding and owner.
func (s *Storage) ClaimFile(ctx context.Context, claimKey, binding string, owner coal.ID) (*File, error) {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.ClaimFile")
	defer span.Finish()

	// get binding
	bnd := s.register.Get(binding)
	if bnd == nil {
		return nil, fmt.Errorf("unknown binding: %s", binding)
	}

	// check owner
	if owner.IsZero() {
		return nil, fmt.Errorf("missing owner")
	}

	// verify claim key
	var key ClaimKey
	err := s.notary.Verify(&key, claimKey)
	if err != nil {
		return nil, err
	}

	// verify limit
	if bnd.Limit > 0 && key.Size > bnd.Limit {
		return nil, fmt.Errorf("too big")
	}

	// verify type
	if len(bnd.Types) > 0 && !stick.Contains(bnd.Types, key.Type) {
		return nil, fmt.Errorf("unsupported type: %s", key.Type)
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
		return nil, fmt.Errorf("unable to claim file")
	}

	return &file, nil
}

// Release will release the link at the field on the provided model. The
// released link must be persisted in the same transaction as the release to
// ensure consistency.
func (s *Storage) Release(ctx context.Context, model coal.Model, field string) error {
	// get field
	value := stick.MustGet(model, field)

	// get link
	var link Link
	switch value := value.(type) {
	case Link:
		link = value
	case *Link:
		link = *value
	}

	// release link
	err := s.ReleaseLink(ctx, &link)
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
func (s *Storage) ReleaseLink(ctx context.Context, link *Link) error {
	// get file
	file := link.File
	if file == nil || file.IsZero() {
		return fmt.Errorf("invalid file id")
	}

	// check transaction
	if !coal.HasTransaction(ctx) {
		return fmt.Errorf("missing transaction for release")
	}

	// release file
	err := s.ReleaseFile(ctx, *file)
	if err != nil {
		return err
	}

	return nil
}

// ReleaseFile will release the file with the provided id.
func (s *Storage) ReleaseFile(ctx context.Context, file coal.ID) error {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.ReleaseFile")
	defer span.Finish()

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
		return fmt.Errorf("unable to release file")
	}

	return nil
}

// Modifier will handle modifications on all or just the specified link fields
// on the model.
func (s *Storage) Modifier(fields ...string) *fire.Callback {
	return fire.C("blaze/Storage.Modifier", fire.Only(fire.Create, fire.Update, fire.Delete), func(ctx *fire.Context) error {
		// check store
		if ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
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
				return fmt.Errorf("missing binding")
			}

			// inspect type
			var err error
			switch value := value.(type) {
			case Link:
				var oldLink *Link
				if oldValue != nil {
					l := oldValue.(Link)
					oldLink = &l
				}
				newLink := &value
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}
				err = s.modifyLink(ctx, newLink, oldLink, binding.Name, owner)
				stick.MustSet(ctx.Model, field, value)
			case *Link:
				var oldLink *Link
				if oldValue != nil {
					oldLink = oldValue.(*Link)
				}
				newLink := value
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}
				err = s.modifyLink(ctx, newLink, oldLink, binding.Name, owner)
				stick.MustSet(ctx.Model, field, newLink)
			default:
				err = fmt.Errorf("%s: unsupported type: %T", field, value)
			}
			if err != nil {
				return fmt.Errorf("%s: %w", field, err)
			}
		}

		return nil
	})
}

func (s *Storage) modifyLink(ctx context.Context, newLink, oldLink *Link, binding string, owner coal.ID) error {
	// detect change
	added := oldLink == nil && newLink != nil
	updated := oldLink != nil && newLink != nil && newLink.ClaimKey != ""
	deleted := oldLink != nil && newLink == nil

	// check if changed
	if !added && !updated && !deleted {
		return nil
	}

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
		newLink.File = nil

		// claim
		err := s.ClaimLink(ctx, newLink, binding, owner)
		if err != nil {
			return err
		}
	}

	return nil
}

// Decorate will populate the view key of the provided link if a file is
// available.
func (s *Storage) Decorate(link *Link) error {
	// skip if file is missing
	if link == nil || link.File == nil || link.File.IsZero() {
		return nil
	}

	// issue view key
	viewKey, err := s.notary.Issue(&ViewKey{
		File: *link.File,
	})
	if err != nil {
		return err
	}

	// set key
	link.ViewKey = viewKey

	// copy info
	link.Type = link.FileType
	link.Size = link.FileSize

	return nil
}

// Decorator will generate view keys for all or just the specified link fields
// on the returned model or models.
func (s *Storage) Decorator(fields ...string) *fire.Callback {
	return fire.C("blaze/Storage.Decorator", fire.All(), func(ctx *fire.Context) error {
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

func (s *Storage) decorateModel(model coal.Model, fields []string) error {
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
			err = s.Decorate(&value)
			stick.MustSet(model, field, value)
		case *Link:
			err = s.Decorate(value)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// Download will initiate a download for the file referenced by the provided
// view key.
func (s *Storage) Download(ctx context.Context, viewKey string) (Download, *File, error) {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.Download")
	defer span.Finish()

	// verify key
	var key ViewKey
	err := s.notary.Verify(&key, viewKey)
	if err != nil {
		return nil, nil, err
	}

	// find file
	var file File
	found, err := s.store.M(&File{}).FindFirst(ctx, &file, bson.M{
		"_id":   key.File,
		"State": Claimed,
	}, nil, 0, false)
	if err != nil {
		return nil, nil, err
	} else if !found {
		return nil, nil, fmt.Errorf("missing file")
	}

	// begin download
	download, err := s.service.Download(ctx, file.Handle)
	if err != nil {
		return nil, nil, err
	}

	return download, &file, nil
}

// DownloadAction returns an action that allows downloading files using view
// keys. This action is usually publicly accessible.
func (s *Storage) DownloadAction() *fire.Action {
	return fire.A("blaze/Storage.DownloadAction", []string{"HEAD", "GET"}, 0, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
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
			return fmt.Errorf("missing binding")
		}

		// prepare content disposition
		contentDisposition := "inline"
		if dl {
			contentDisposition = "attachment"
		}

		// append file to disposition if present
		if binding.Filename != "" {
			contentDisposition = fmt.Sprintf(`%s; filename="%s"`, contentDisposition, binding.Filename)
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
func (s *Storage) CleanupTask(lifetime, timeout, periodicity, retention time.Duration) *axe.Task {
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
// "deleting" are removed immediately. It will also allow the service to cleanup.
func (s *Storage) Cleanup(ctx context.Context, retention time.Duration) error {
	// set default retention
	if retention == 0 {
		retention = time.Hour
	}

	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.Cleanup")
	span.Log("retention", retention.String())
	defer span.Finish()

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
	}, nil, 0, 0, false, coal.Unsafe)
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
		deleted, err := s.service.Delete(ctx, file.Handle)
		if err != nil {
			return err
		}

		// delete file if blob has been deleted
		if deleted {
			_, err = s.store.M(&File{}).Delete(ctx, nil, file.ID())
			if err != nil {
				return err
			}
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
		return err
	}

	return nil
}
