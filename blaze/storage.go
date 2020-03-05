package blaze

import (
	"context"
	"errors"
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
)

// ErrInvalidHandle is returned if the provided handle is invalid.
var ErrInvalidHandle = errors.New("invalid handle")

// ErrUsedHandle is returned if the provided handle has already been used.
var ErrUsedHandle = errors.New("used handle")

// ErrNotFound is returned if there is no blob for the provided handle.
var ErrNotFound = errors.New("not found")

// Handle is a reference to a blob stored in a service.
type Handle map[string]interface{}

// Service is responsible for managing blobs.
type Service interface {
	// Prepare should return a new handle for uploading a blob.
	Prepare(ctx context.Context) (Handle, error)

	// Upload should persist a blob with data read from the provided reader.
	Upload(ctx context.Context, handle Handle, contentType string, r io.Reader) (int64, error)

	// Download lookup the blob and stream its content to the provider writer.
	Download(ctx context.Context, handle Handle, w io.Writer) error

	// Delete should delete the blob.
	Delete(ctx context.Context, handle Handle) (bool, error)

	// Cleanup is called periodically and allows the service to cleanup its
	// storage until the context is cancelled.
	Cleanup(ctx context.Context) error
}

// Storage provides file storage services.
type Storage struct {
	store   *coal.Store
	notary  *heat.Notary
	service Service
}

// NewStorage creates a new storage.
func NewStorage(store *coal.Store, notary *heat.Notary, service Service) *Storage {
	return &Storage{
		store:   store,
		notary:  notary,
		service: service,
	}
}

// UploadAction returns an action that provides and upload that service that
// stores blobs and returns claim keys.
func (s *Storage) UploadAction(limit int64) *fire.Action {
	// set default limit
	if limit == 0 {
		limit = serve.MustByteSize("4M")
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
			keys, err = s.uploadRaw(ctx, contentType, contentLength)
		}

		// check limit error
		if err != nil && strings.HasSuffix(err.Error(), serve.ErrBodyLimitExceeded.Error()) {
			ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)
			return nil
		}

		// respond with keys
		return ctx.Respond(fire.Map{
			"keys": keys,
		})
	})
}

func (s *Storage) uploadRaw(ctx *fire.Context, contentType string, contentLength int64) ([]string, error) {
	// trace
	ctx.Trace.Push("blaze/Storage.uploadRaw")
	defer ctx.Trace.Pop()

	// set default content type
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// upload file to service
	file, err := s.upload(ctx, contentType, contentLength, ctx.HTTPRequest.Body)
	if err != nil {
		return nil, err
	}

	// issue claim key
	claimKey, err := s.notary.Issue(&ClaimKey{
		File: file.ID(),
	})
	if err != nil {
		return nil, err
	}

	return []string{claimKey}, nil
}

func (s *Storage) uploadMultipart(ctx *fire.Context, boundary string) ([]string, error) {
	// trace
	ctx.Trace.Push("blaze/Storage.uploadMultipart")
	defer ctx.Trace.Pop()

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

		// upload file to service
		file, err := s.upload(ctx, contentType, -1, part)
		if err != nil {
			return nil, err
		}

		// issue claim key
		claimKey, err := s.notary.Issue(&ClaimKey{
			File: file.ID(),
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

func (s *Storage) upload(ctx context.Context, contentType string, length int64, stream io.Reader) (*File, error) {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.upload")
	span.Log("contentType", contentType)
	span.Log("length", length)
	defer span.Finish()

	// limit upload if length has been specified
	if length != -1 {
		stream = io.LimitReader(stream, length)
	}

	// create handle
	handle, err := s.service.Prepare(ctx)
	if err != nil {
		return nil, err
	}

	// prepare file
	file := &File{
		Base:    coal.B(),
		State:   Uploading,
		Updated: time.Now(),
		Type:    contentType,
		Handle:  handle,
	}

	// create file
	err = s.store.M(file).Insert(ctx, file)
	if err != nil {
		return nil, err
	}

	// upload file to service
	length, err = s.service.Upload(ctx, handle, contentType, stream)
	if err != nil {
		return nil, err
	}

	// update file
	_, err = s.store.M(file).Update(ctx, file.ID(), bson.M{
		"$set": bson.M{
			"State":   Uploaded,
			"Updated": time.Now(),
			"Length":  length,
		},
	}, false)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Validator will validate all or just the specified link fields of the model.
func (s *Storage) Validator(fields ...string) *fire.Callback {
	return fire.C("blaze/Storage.Validator", fire.Only(fire.Create, fire.Update, fire.Delete), func(ctx *fire.Context) error {
		// check store
		if ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
		}

		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// check all fields
		for _, field := range fields {
			// get value
			value := coal.MustGet(ctx.Model, field)

			// get old value
			var oldValue interface{}
			if ctx.Original != nil {
				oldValue = coal.MustGet(ctx.Original, field)
			}

			// get path
			path := coal.GetMeta(ctx.Model).Fields[field].JSONKey

			// inspect type
			var err error
			switch value := value.(type) {
			case Link:
				var oldLink *Link
				if oldValue != nil {
					r := oldValue.(Link)
					oldLink = &r
				}
				newLink := &value
				if ctx.Operation == fire.Delete {
					oldLink = newLink
					newLink = nil
				}
				err = s.validateLink(ctx, newLink, oldLink, path)
				coal.MustSet(ctx.Model, field, value)
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
				err = s.validateLink(ctx, newLink, oldLink, path)
				coal.MustSet(ctx.Model, field, newLink)
			default:
				err = fmt.Errorf("unsupported type")
			}
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Storage) validateLink(ctx context.Context, newLink, oldLink *Link, path string) error {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.validateLink")
	defer span.Finish()

	// detect change
	added := oldLink == nil && newLink != nil
	updated := oldLink != nil && newLink != nil && newLink.ClaimKey != ""
	deleted := oldLink != nil && newLink == nil

	// check if changed
	if !added && !updated && !deleted {
		return nil
	}

	// claim new file
	if added || updated {
		// check claim
		if newLink.ClaimKey == "" {
			return fmt.Errorf("%s: missing claim key", path)
		}

		// verify claim key
		var claimKey ClaimKey
		err := s.notary.Verify(&claimKey, newLink.ClaimKey)
		if err != nil {
			return err
		}

		// TODO: Optimize with a find and update operation.

		// get new file
		var newFile File
		found, err := s.store.M(&newFile).Find(ctx, &newFile, claimKey.File, true)
		if err != nil {
			return err
		} else if !found {
			return fmt.Errorf("to be claimed file is missing")
		}

		// check new file state
		if newFile.State != Uploaded {
			return fmt.Errorf("%s: invalid file state", path)
		}

		// claim new file
		found, err = s.store.M(&File{}).UpdateFirst(ctx, bson.M{
			"_id":   newFile.ID(),
			"State": Uploaded,
		}, bson.M{
			"$set": bson.M{
				"State":   Claimed,
				"Updated": time.Now(),
			},
		}, false)
		if err != nil {
			return err
		} else if !found {
			return fmt.Errorf("%s: unable to claim file", path)
		}

		// update link
		newLink.Type = newFile.Type
		newLink.Length = newFile.Length
		newLink.File = coal.P(newFile.ID())
	}

	// release old file
	if updated || deleted {
		found, err := s.store.M(&File{}).UpdateFirst(ctx, bson.M{
			"_id":   oldLink.File,
			"State": Claimed,
		}, bson.M{
			"$set": bson.M{
				"State":   Released,
				"Updated": time.Now(),
			},
		}, false)
		if err != nil {
			return err
		} else if !found {
			return fmt.Errorf("%s: unable to release file", path)
		}
	}

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
		value := coal.MustGet(model, field)

		// inspect type
		var err error
		switch value := value.(type) {
		case Link:
			err = s.decorateLink(&value)
			coal.MustSet(model, field, value)
		case *Link:
			err = s.decorateLink(value)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) decorateLink(link *Link) error {
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

	return nil
}

// DownloadAction returns an action that allows downloading files using view
// keys.
func (s *Storage) DownloadAction() *fire.Action {
	return fire.A("blaze/Storage.DownloadAction", []string{"GET"}, 0, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.store {
			return fmt.Errorf("stores must be identical")
		}

		// verify key
		var key ViewKey
		err := s.notary.Verify(&key, ctx.HTTPRequest.URL.Query().Get("key"))
		if err != nil {
			return err
		}

		// load file
		var file File
		found, err := s.store.M(&File{}).FindFirst(ctx, &file, bson.M{
			"_id":   key.File,
			"State": Claimed,
		}, nil, 0, false)
		if err != nil {
			return err
		} else if !found {
			return fmt.Errorf("missing file")
		}

		// set content type and length
		ctx.ResponseWriter.Header().Set("Content-Type", file.Type)
		ctx.ResponseWriter.Header().Set("Content-Length", strconv.FormatInt(file.Length, 10))

		// download file
		err = s.service.Download(ctx, file.Handle, ctx.ResponseWriter)
		if err != nil {
			return err
		}

		return nil
	})
}

// CleanupTask will return a periodic task that can be run to periodically
// cleanup obsolete files.
func (s *Storage) CleanupTask(periodicity, retention time.Duration) *axe.Task {
	return &axe.Task{
		Name: "fire/blaze.cleanup",
		Handler: func(ctx *axe.Context) error {
			return s.Cleanup(ctx, retention)
		},
		Workers:      1,
		MaxAttempts:  1,
		Lifetime:     periodicity,
		Periodically: periodicity,
		PeriodicJob: axe.Blueprint{
			Label: "periodic",
		},
	}
}

// Cleanup will remove obsolete files and remove their blobs. Files in the
// states "uploading" or "uploaded" are removed after the specified retention
// which defaults to 1 hour if zero. Files in the states "released" and
// "deleting" are removed immediately. It will also allow the service to cleanup.
func (s *Storage) Cleanup(ctx context.Context, retention time.Duration) error {
	// track
	ctx, span := cinder.Track(ctx, "blaze/Storage.Cleanup")
	span.Log("retention", retention.String())
	defer span.Finish()

	// set default retention
	if retention == 0 {
		retention = time.Hour
	}

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
			found, err := s.store.M(&File{}).UpdateFirst(ctx, bson.M{
				"_id":   file.ID(),
				"State": file.State,
			}, bson.M{
				"$set": bson.M{
					"State":   Deleting,
					"Updated": time.Now(),
				},
			}, false)
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
