package blaze

import (
	"context"
	"io"
	"math/rand"
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
	bindings *Registry
	services map[string]Service
	uploader []string
}

// NewBucket creates a new bucket from a store, notary and binding registry.
func NewBucket(store *coal.Store, notary *heat.Notary, bindings ...*Binding) *Bucket {
	return &Bucket{
		store:    store,
		notary:   notary,
		bindings: NewRegistry(bindings...),
		services: map[string]Service{},
	}
}

// Use will register the specified service using the provided name. If upload
// is true the service is used for new uploads.
func (b *Bucket) Use(service Service, name string, upload bool) {
	// check existence
	if b.services[name] != nil {
		panic("duplicate service name")
	}

	// store service
	b.services[name] = service

	// add uploader
	if upload {
		b.uploader = append(b.uploader, name)
	}
}

// Upload will initiate and perform an upload using the provided callback and
// return a claim key and the uploaded file. Upload must be called outside a
// transaction to ensure the uploaded file is tracked in case of errors.
func (b *Bucket) Upload(ctx context.Context, name, mediaType string, size int64, cb func(Upload) (int64, error)) (string, *File, error) {
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
		mediaType = "application/octet-stream"
		if name != "" {
			mediaType = serve.MimeTypeByExtension(path.Ext(name), false)
		}
	}

	// check uploader
	if len(b.uploader) == 0 {
		return "", nil, xo.F("no uploader services configured")
	}

	// select random uploader
	uploader := b.uploader[rand.Intn(len(b.uploader))]

	// get service
	service := b.services[uploader]

	// create handle
	handle, err := service.Prepare(ctx)
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
		Size:    size,
		Service: uploader,
		Handle:  handle,
	}

	// validate file
	err = file.Validate()
	if err != nil {
		return "", nil, err
	}

	// create file
	err = b.store.M(file).Insert(ctx, file)
	if err != nil {
		return "", nil, err
	}

	// begin upload
	upload, err := service.Upload(ctx, handle, Info{
		Size:      size,
		MediaType: mediaType,
	})
	if err != nil {
		return "", nil, xo.W(err)
	}

	// perform upload and ensure abort or close
	uploadSize, err := cb(upload)
	if err != nil {
		_ = upload.Abort()
		return "", nil, xo.W(err)
	}
	_ = upload.Close()

	// verify size
	if uploadSize != size {
		return "", nil, xo.SF("size mismatch")
	}

	// verify upload meta data
	info, err := service.Lookup(ctx, handle)
	if err != nil {
		return "", nil, xo.W(err)
	} else if info.Size != size {
		return "", nil, xo.F("upload verification failed")
	}

	// verify upload data
	download, err := service.Download(ctx, handle)
	if err != nil {
		return "", nil, xo.W(err)
	}
	defer download.Close()
	if size > 0 {
		_, err = download.Seek(size-1, io.SeekStart)
		if err != nil {
			return "", nil, xo.W(err)
		}
		n, err := download.Read(make([]byte, 1))
		if err != nil {
			return "", nil, xo.W(err)
		} else if n != 1 {
			return "", nil, xo.F("upload verification failed")
		}
	}
	err = download.Close()
	if err != nil {
		return "", nil, xo.W(err)
	}

	// get time
	now := time.Now()

	// set fields
	file.State = Uploaded
	file.Updated = now

	// validate file
	err = file.Validate()
	if err != nil {
		return "", nil, err
	}

	// update file
	_, err = b.store.M(file).Update(ctx, file, file.ID(), bson.M{
		"$set": bson.M{
			"State":   Uploaded,
			"Updated": now,
		},
	}, false)
	if err != nil {
		return "", nil, err
	}

	// issue claim key
	claimKey, err := b.notary.Issue(ctx, &ClaimKey{
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
func (b *Bucket) UploadAction(limit int64, timeout time.Duration) *fire.Action {
	// set default limit
	if limit == 0 {
		limit = serve.MustByteSize("8M")
	}

	// set default timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	return fire.A("blaze/Bucket.UploadAction", []string{"POST"}, limit, timeout, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != b.store {
			return xo.F("stores must be identical")
		}

		// get raw content type
		rawContentType := ctx.HTTPRequest.Header.Get("Content-Type")

		// get content type
		contentType, ctParams, err := mime.ParseMediaType(rawContentType)
		if rawContentType != "" && err != nil {
			ctx.ResponseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = ctx.ResponseWriter.Write([]byte("invalid content type"))
			return nil
		}

		// check content length
		contentLength := ctx.HTTPRequest.ContentLength
		if contentLength != -1 && contentLength > limit {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		}

		// upload multipart or raw
		var keys []string
		if contentType == "multipart/form-data" {
			keys, err = b.uploadMultipart(ctx, ctParams["boundary"])
		} else {
			keys, err = b.uploadBody(ctx, contentType)
		}

		// handle error
		if err != nil && strings.HasSuffix(err.Error(), serve.ErrBodyLimitExceeded.Error()) {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		} else if err != nil && xo.IsSafe(err) {
			ctx.ResponseWriter.WriteHeader(http.StatusBadRequest)
			_, _ = ctx.ResponseWriter.Write([]byte(err.Error()))
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

func (b *Bucket) uploadBody(ctx *fire.Context, mediaType string) ([]string, error) {
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

	// parse content length
	contentLength, err := strconv.ParseInt(ctx.HTTPRequest.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, xo.SF("invalid content length")
	}

	// upload stream
	claimKey, _, err := b.Upload(ctx, filename, mediaType, contentLength, func(upload Upload) (int64, error) {
		return UploadFrom(upload, ctx.HTTPRequest.Body)
	})
	if err != nil {
		return nil, err
	}

	return []string{claimKey}, nil
}

func (b *Bucket) uploadMultipart(ctx *fire.Context, boundary string) ([]string, error) {
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
			return nil, xo.SF("invalid content type")
		}

		// parse content length
		contentLength, err := strconv.ParseInt(part.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return nil, xo.SF("invalid content length")
		}

		// upload part
		claimKey, _, err := b.Upload(ctx, part.FileName(), contentType, contentLength, func(upload Upload) (int64, error) {
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
func (b *Bucket) Claim(ctx context.Context, model coal.Model, field string) error {
	// get value
	value := stick.MustGet(model, field)

	// lookup binding
	binding, _ := b.bindings.Get(&Binding{
		Model: model,
		Field: field,
	})
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
	err := b.ClaimLink(ctx, link, binding.Name, model.ID())
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
func (b *Bucket) ClaimLink(ctx context.Context, link *Link, binding string, owner coal.ID) error {
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
	file, err := b.ClaimFile(ctx, link.ClaimKey, binding, owner)
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
//
// Note: For consistency this should be called within a transaction.
func (b *Bucket) ClaimFile(ctx context.Context, claimKey, binding string, owner coal.ID) (*File, error) {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.ClaimFile")
	defer span.End()

	// get binding
	bnd, _ := b.bindings.Get(&Binding{Name: binding})
	if bnd == nil {
		return nil, xo.F("unknown binding: %s", binding)
	}

	// check owner
	if owner.IsZero() {
		return nil, xo.F("missing owner")
	}

	// verify claim key
	var key ClaimKey
	err := b.notary.Verify(ctx, &key, claimKey)
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
	found, err := b.store.M(&File{}).UpdateFirst(ctx, &file, bson.M{
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
func (b *Bucket) Release(ctx context.Context, model coal.Model, field string) error {
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
	err := b.ReleaseLink(ctx, link)
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
func (b *Bucket) ReleaseLink(ctx context.Context, link *Link) error {
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
	err := b.ReleaseFile(ctx, file)
	if err != nil {
		return err
	}

	return nil
}

// ReleaseFile will release the file with the provided id.
//
// Note: For consistency this should be called within a transaction.
func (b *Bucket) ReleaseFile(ctx context.Context, file coal.ID) error {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.ReleaseFile")
	defer span.End()

	// release file
	found, err := b.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
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
func (b *Bucket) Modifier(fields ...string) *fire.Callback {
	return fire.C("blaze/Bucket.Modifier", fire.Modifier, fire.Only(fire.Create|fire.Update|fire.Delete), func(ctx *fire.Context) error {
		// check store
		if ctx.Store != b.store {
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
			binding, _ := b.bindings.Get(&Binding{
				Model: ctx.Model,
				Field: field,
			})
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
				err := b.modifyLink(ctx, newLink, oldLink, binding.Name, owner)
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
				err := b.modifyLink(ctx, newLink, oldLink, binding.Name, owner)
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
				err := b.modifyLinks(ctx, newLinks, oldLinks, binding.Name, owner)
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

func (b *Bucket) modifyLink(ctx context.Context, newLink, oldLink *Link, binding string, owner coal.ID) error {
	// detect change
	added := oldLink == nil && newLink != nil
	updated := oldLink != nil && newLink != nil && newLink.ClaimKey != ""
	deleted := oldLink != nil && newLink == nil

	// release old file
	if updated || deleted {
		err := b.ReleaseLink(ctx, oldLink)
		if err != nil {
			return err
		}
	}

	// claim new file
	if added || updated {
		// unset file
		newLink.File = coal.ID{}

		// claim
		err := b.ClaimLink(ctx, newLink, binding, owner)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Bucket) modifyLinks(ctx context.Context, newLinks, oldLinks Links, binding string, owner coal.ID) error {
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
		err := b.ReleaseLink(ctx, oldMap[ref])
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
		err := b.ClaimLink(ctx, link, binding, owner)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetViewKey generates and returns a view key for the specified file.
func (b *Bucket) GetViewKey(ctx context.Context, file coal.ID) (string, error) {
	// the view key is generated in such way that the key is stable for at least
	// half of the expiry duration. this enables browsers to cache downloads

	// get issued and expires
	expiry := heat.GetMeta(&ViewKey{}).Expiry
	issued := time.Now().Truncate(expiry / 2)
	expires := issued.Add(expiry)

	// issue view key
	viewKey, err := b.notary.Issue(ctx, &ViewKey{
		Base: heat.Base{
			ID:      file,
			Issued:  issued,
			Expires: expires,
		},
		File: file,
	})
	if err != nil {
		return "", err
	}

	return viewKey, nil
}

// Decorate will populate the provided link if a file is available.
func (b *Bucket) Decorate(ctx context.Context, link *Link) error {
	// skip if file is missing
	if link == nil || link.File.IsZero() {
		return nil
	}

	// get view key
	viewKey, err := b.GetViewKey(ctx, link.File)
	if err != nil {
		return err
	}

	// set view key
	link.ViewKey = viewKey

	// copy info
	link.Name = link.FileName
	link.Type = link.FileType
	link.Size = link.FileSize

	return nil
}

// Decorator will populate all or just the specified link fields.
func (b *Bucket) Decorator(fields ...string) *fire.Callback {
	return fire.C("blaze/Bucket.Decorator", fire.Decorator, fire.All(), func(ctx *fire.Context) error {
		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// decorate model
		if ctx.Model != nil {
			err := b.decorateModel(ctx, ctx.Model, fields)
			if err != nil {
				return err
			}
		}

		// decorate models
		for _, model := range ctx.Models {
			err := b.decorateModel(ctx, model, fields)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (b *Bucket) decorateModel(ctx context.Context, model coal.Model, fields []string) error {
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
			err = b.Decorate(ctx, &value)
			if err != nil {
				return err
			}

			// update link
			stick.MustSet(model, field, value)
		case *Link:
			// decorate link
			err = b.Decorate(ctx, value)
			if err != nil {
				return err
			}
		case Links:
			// decorate links
			for i := range value {
				err = b.Decorate(ctx, &value[i])
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
func (b *Bucket) Download(ctx context.Context, viewKey string) (Download, *File, error) {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.Download")
	defer span.End()

	// verify key
	var key ViewKey
	err := b.notary.Verify(ctx, &key, viewKey)
	if err != nil {
		return nil, nil, err
	}

	// download file
	download, file, err := b.DownloadFile(ctx, key.File)
	if err != nil {
		return nil, nil, err
	}

	return download, file, nil
}

// DownloadFile will initiate a download for the specified file.
func (b *Bucket) DownloadFile(ctx context.Context, id coal.ID) (Download, *File, error) {
	// find file
	var file File
	found, err := b.store.M(&File{}).FindFirst(ctx, &file, bson.M{
		"_id":   id,
		"State": Claimed,
	}, nil, 0, false)
	if err != nil {
		return nil, nil, err
	} else if !found {
		return nil, nil, xo.F("missing file")
	}

	// get service
	service := b.services[file.Service]
	if service == nil {
		return nil, nil, xo.F("unknown service: %s", file.Service)
	}

	// begin download
	download, err := service.Download(ctx, file.Handle)
	if err != nil {
		return nil, nil, xo.W(err)
	}

	return download, &file, nil
}

// DownloadAction returns an endpoint that allows downloading files using view
// keys. This action is usually publicly accessible.
func (b *Bucket) DownloadAction(timeout time.Duration) *fire.Action {
	// set default timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	return fire.A("blaze/Bucket.DownloadAction", []string{"HEAD", "GET"}, 0, timeout, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != b.store {
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
		download, file, err := b.Download(ctx, key)
		if err != nil {
			return err
		}

		// get binding
		binding, _ := b.bindings.Get(&Binding{Name: file.Binding})
		if binding == nil {
			return xo.F("missing binding")
		}

		// prepare content disposition
		contentDisposition := "inline"
		if dl {
			// get filename
			filename := file.Name
			if binding.FileName != "" {
				filename = binding.FileName
			} else if filename == "" {
				filename = file.ID().Hex()
			}

			// set disposition
			contentDisposition = mime.FormatMediaType("attachment", map[string]string{
				"filename": filename,
			})
		}

		// set content type, length and disposition
		ctx.ResponseWriter.Header().Set("Content-Type", file.Type)
		ctx.ResponseWriter.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
		ctx.ResponseWriter.Header().Set("Content-Disposition", contentDisposition)

		// unset any content security policy
		ctx.ResponseWriter.Header().Del("Content-Security-Policy")

		// cache download for one year, using a versioned ETag based on the file ID
		ctx.ResponseWriter.Header().Set("ETag", `"v1-`+file.ID().Hex()+`"`)
		ctx.ResponseWriter.Header().Set("Cache-Control", "public, max-age=31536000")

		// stream download
		http.ServeContent(ctx.ResponseWriter, ctx.HTTPRequest, "", file.Updated, download)

		return nil
	})
}

// CleanupFile will clean up a single file. In the first step, files in the
// uploading or uploaded state are marked as "deleting". In the second step,
// blobs of "deleting" files are deleted. In the last step "deleting" files with
// missing blobs are fully deleted.
func (b *Bucket) CleanupFile(ctx context.Context, id coal.ID) error {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.CleanupFile")
	span.Tag("id", id.Hex())
	defer span.End()

	// get file
	var file File
	found, err := b.store.M(&file).Find(ctx, &file, id, false)
	if err != nil {
		return err
	} else if !found {
		return xo.F("missing file")
	}

	// mark leftover uploaded files
	if file.State == Uploading || file.State == Uploaded || file.State == Released {
		found, err := b.store.M(&file).UpdateFirst(ctx, &file, bson.M{
			"_id":   file.ID(),
			"State": file.State,
		}, bson.M{
			"$set": bson.M{
				"State": Deleting,
			},
		}, nil, false)
		if err != nil {
			return err
		} else if !found {
			return xo.F("missing file")
		}

		return nil
	}

	// check state
	if file.State != Deleting {
		return xo.F("unexpected state: %s", file.State)
	}

	// get service
	service := b.services[file.Service]
	if service == nil {
		return xo.F("unknown service: %s", file.Service)
	}

	// delete blob
	err = service.Delete(ctx, file.Handle)
	if err != nil && !ErrNotFound.Is(err) {
		return err
	}

	// return if blob is not yet absent
	if !ErrNotFound.Is(err) {
		return nil
	}

	// otherwise, delete file
	_, err = b.store.M(&File{}).Delete(ctx, nil, file.ID())
	if err != nil {
		return err
	}

	return nil
}

// CleanupTask will return a periodic task that will scan and enqueue jobs that
// clean up files in the "uploading" or "uploaded" state older than the specified
// retention as well as files in the "released" and "deleting" state.
func (b *Bucket) CleanupTask(retention time.Duration, batch int) *axe.Task {
	// set default retention and batch
	if retention == 0 {
		retention = time.Hour
	}
	if batch == 0 {
		batch = 100
	}

	return &axe.Task{
		Job: &CleanupJob{},
		Handler: func(ctx *axe.Context) error {
			// get job
			job := ctx.Job.(*CleanupJob)

			// handle file
			if job.Label != "scan" {
				// handle migration
				id, err := coal.FromHex(job.Label)
				if err != nil {
					return err
				}

				// clean up file
				err = b.CleanupFile(ctx, id)
				if err != nil {
					return err
				}

				return nil
			}

			/* scan files  */

			// get files
			var files []File
			err := b.store.M(&File{}).FindAll(ctx, &files, bson.M{
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
			}, nil, 0, int64(batch), false, coal.NoTransaction)
			if err != nil {
				return err
			}

			// enqueue jobs
			for _, file := range files {
				_, err = ctx.Queue.Enqueue(ctx, &CleanupJob{
					Base: axe.B(file.ID().Hex()),
				}, 0, 0)
				if err != nil {
					return err
				}
			}

			return nil
		},
		Workers:     1,
		MaxAttempts: 1,
		Lifetime:    time.Minute,
		Timeout:     2 * time.Minute,
		Periodicity: 5 * time.Minute,
		PeriodicJob: axe.Blueprint{
			Job: &CleanupJob{
				Base: axe.B("scan"),
			},
		},
	}
}

// MigrateFile will migrate a single file by re-uploading the specified files
// blob using on of the configured uploader services and swapping the handle to
// the original file. On success or error the original or temporary blob is
// automatically cleaned up.
func (b *Bucket) MigrateFile(ctx context.Context, id coal.ID) error {
	// trace
	ctx, span := xo.Trace(ctx, "blaze/Bucket.MigrateFile")
	span.Tag("id", id.Hex())
	defer span.End()

	// download original file
	download, original, err := b.DownloadFile(ctx, id)
	if err != nil {
		return err
	} else if original.State != Claimed {
		return xo.F("unexpected file state")
	}

	// ensure download is closed
	defer download.Close()

	// upload new file
	_, newFile, err := b.Upload(ctx, original.Name, original.Type, original.Size, func(upload Upload) (int64, error) {
		return UploadFrom(upload, download)
	})
	if err != nil {
		return err
	} else if newFile.State != Uploaded {
		return xo.F("unexpected file state")
	}

	// check services
	if original.Service == newFile.Service {
		return xo.F("unexpected service match")
	}

	// swap services and handles
	err = b.store.T(ctx, false, func(ctx context.Context) error {
		// update original file
		found, err := b.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
			"_id":     original.ID(),
			"State":   original.State,
			"Service": original.Service,
			"Handle":  original.Handle,
		}, bson.M{
			"$set": bson.M{
				"Updated": time.Now(),
				"Service": newFile.Service,
				"Handle":  newFile.Handle,
			},
		}, nil, false)
		if err != nil {
			return err
		} else if !found {
			return xo.F("file not found")
		}

		// update new file
		found, err = b.store.M(&File{}).UpdateFirst(ctx, nil, bson.M{
			"_id":     newFile.ID(),
			"State":   newFile.State,
			"Service": newFile.Service,
			"Handle":  newFile.Handle,
		}, bson.M{
			"$set": bson.M{
				"State":   Released,
				"Updated": time.Now(),
				"Service": original.Service,
				"Handle":  original.Handle,
			},
		}, nil, false)
		if err != nil {
			return err
		} else if !found {
			return xo.F("file not found")
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// MigrateTask will return a periodic task that will scan and enqueue jobs that
// migrates files using one of the provided services to the current uploader
// services. Up to specified amount of files are migrated per run.
func (b *Bucket) MigrateTask(services []string, batch int) *axe.Task {
	// set default batch
	if batch == 0 {
		batch = 100
	}

	return &axe.Task{
		Job: &MigrateJob{},
		Handler: func(ctx *axe.Context) error {
			// get job
			job := ctx.Job.(*MigrateJob)

			// handle file
			if job.Label != "scan" {
				// handle migration
				id, err := coal.FromHex(job.Label)
				if err != nil {
					return err
				}

				// migrate file
				err = b.MigrateFile(ctx, id)
				if err != nil {
					return err
				}

				return nil
			}

			/* scan files  */

			// get files
			var files []File
			err := b.store.M(&File{}).FindAll(ctx, &files, bson.M{
				"State": Claimed,
				"Service": bson.M{
					"$in": services,
				},
			}, nil, 0, int64(batch), false, coal.NoTransaction)
			if err != nil {
				return err
			}

			// enqueue jobs
			for _, file := range files {
				_, err = ctx.Queue.Enqueue(ctx, &MigrateJob{
					Base: axe.B(file.ID().Hex()),
				}, 0, 0)
				if err != nil {
					return err
				}
			}

			return nil
		},
		Workers:     1,
		MaxAttempts: 1,
		Lifetime:    time.Minute,
		Timeout:     2 * time.Minute,
		Periodicity: 5 * time.Minute,
		PeriodicJob: axe.Blueprint{
			Job: &MigrateJob{
				Base: axe.B("scan"),
			},
		},
	}
}
