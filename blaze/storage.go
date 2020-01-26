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
	Prepare() (Handle, error)

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

type Storage struct {
	// The store used to manage the files.
	Store *coal.Store

	// The notary used for handling keys.
	Notary *heat.Notary

	// The service used for storing and retrieving blobs.
	Service Service
}

func (s *Storage) Upload(limit int64) *fire.Action {
	// set default limit
	if limit == 0 {
		limit = serve.MustByteSize("4M")
	}

	return fire.A("blaze/Upload", []string{"POST"}, limit, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.Store {
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
	// begin trace
	ctx.Tracer.Push("blaze/uploadRaw")

	// set default content type
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// upload file to service
	file, err := s.Persist(ctx, contentType, contentLength, ctx.HTTPRequest.Body)
	if err != nil {
		return nil, err
	}

	// issue claim key
	claimKey, err := s.Notary.Issue(&ClaimKey{
		File: file.ID(),
	})
	if err != nil {
		return nil, err
	}

	// finish trace
	ctx.Tracer.Pop()

	return []string{claimKey}, nil
}

func (s *Storage) uploadMultipart(ctx *fire.Context, boundary string) ([]string, error) {
	// begin trace
	ctx.Tracer.Push("blaze/uploadMultipart")

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
		file, err := s.Persist(ctx, contentType, -1, part)
		if err != nil {
			return nil, err
		}

		// issue claim key
		claimKey, err := s.Notary.Issue(&ClaimKey{
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

	// finish trace
	ctx.Tracer.Pop()

	return claimKeys, nil
}

// Persist will store a new file in the provided storage.
func (s *Storage) Persist(ctx context.Context, contentType string, length int64, stream io.Reader) (*File, error) {
	// limit upload if length has been specified
	if length != -1 {
		stream = io.LimitReader(stream, length)
	}

	// create handle
	handle, err := s.Service.Prepare()
	if err != nil {
		return nil, err
	}

	// prepare file
	file := coal.Init(&File{
		State:   Uploading,
		Updated: time.Now(),
		Type:    contentType,
		Handle:  handle,
	}).(*File)

	// create file
	_, err = s.Store.C(file).InsertOne(ctx, file)
	if err != nil {
		return nil, err
	}

	// upload file to service
	length, err = s.Service.Upload(ctx, handle, contentType, stream)
	if err != nil {
		return nil, err
	}

	// update file
	_, err = s.Store.C(file).UpdateOne(ctx, bson.M{
		"_id": file.ID(),
	}, bson.M{
		"$set": bson.M{
			coal.F(file, "State"):   Uploaded,
			coal.F(file, "Updated"): time.Now(),
			coal.F(file, "Length"):  length,
		},
	})
	if err != nil {
		return nil, err
	}

	return file, nil
}

// Validator will validate all or just the specified link fields of the model.
func (s *Storage) Validator(fields ...string) *fire.Callback {
	return fire.C("blaze/Storage.Validator", fire.Only(fire.Create, fire.Update, fire.Delete), func(ctx *fire.Context) error {
		// check store
		if ctx.Store != s.Store {
			return fmt.Errorf("stores must be identical")
		}

		// collect fields if empty
		if len(fields) == 0 {
			fields = collectFields(ctx.Controller.Model)
		}

		// check all fields
		for _, field := range fields {
			// get value
			value := ctx.Model.MustGet(field)

			// get old value
			var oldValue interface{}
			if ctx.Original != nil {
				oldValue = ctx.Original.MustGet(field)
			}

			// get path
			path := ctx.Model.Meta().Fields[field].JSONKey

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
				ctx.Model.MustSet(field, value)
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
				ctx.Model.MustSet(field, newLink)
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
		err := s.Notary.Verify(&claimKey, newLink.ClaimKey)
		if err != nil {
			return err
		}

		// get new file
		var newFile File
		err = s.Store.C(&newFile).FindOne(ctx, bson.M{
			"_id": claimKey.File,
		}).Decode(&newFile)
		if err != nil {
			return err
		}

		// check new file state
		if newFile.State != Uploaded {
			return fmt.Errorf("%s: invalid file state", path)
		}

		// claim new file
		res, err := s.Store.C(&File{}).UpdateOne(ctx, bson.M{
			"_id":                    newFile.ID(),
			coal.F(&File{}, "State"): Uploaded,
		}, bson.M{
			"$set": bson.M{
				coal.F(&File{}, "State"):   Claimed,
				coal.F(&File{}, "Updated"): time.Now(),
			},
		})
		if err != nil {
			return err
		} else if res.ModifiedCount != 1 {
			return fmt.Errorf("%s: unable to claim file", path)
		}

		// update link
		newLink.Type = newFile.Type
		newLink.Length = newFile.Length
		newLink.File = coal.P(newFile.ID())
	}

	// release old file
	if updated || deleted {
		res, err := s.Store.C(&File{}).UpdateOne(ctx, bson.M{
			"_id":                    oldLink.File,
			coal.F(&File{}, "State"): Claimed,
		}, bson.M{
			"$set": bson.M{
				coal.F(&File{}, "State"):   Released,
				coal.F(&File{}, "Updated"): time.Now(),
			},
		})
		if err != nil {
			return err
		} else if res.ModifiedCount != 1 {
			return fmt.Errorf("%s: unable to release file", path)
		}
	}

	return nil
}

// Decorator will generate view keys for all or just the specified link fields
// on the returned model or models.
func (s *Storage) Decorator(fields ...string) *fire.Callback {
	return fire.C("blaze.Decorator", fire.All(), func(ctx *fire.Context) error {
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
		value := model.MustGet(field)

		// inspect type
		var err error
		switch value := value.(type) {
		case Link:
			err = s.decorateLink(&value)
			model.MustSet(field, value)
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
	viewKey, err := s.Notary.Issue(&ViewKey{
		File: *link.File,
	})
	if err != nil {
		return err
	}

	// set key
	link.ViewKey = viewKey

	return nil
}

func (s *Storage) Download() *fire.Action {
	return fire.A("blaze/Download", []string{"GET"}, 0, func(ctx *fire.Context) error {
		// check store
		if ctx.Store != nil && ctx.Store != s.Store {
			return fmt.Errorf("stores must be identical")
		}

		// verify key
		var key ViewKey
		err := s.Notary.Verify(&key, ctx.HTTPRequest.URL.Query().Get("key"))
		if err != nil {
			return err
		}

		// load file
		var file File
		err = s.Store.C(&File{}).FindOne(ctx, bson.M{
			"_id": key.File,
		}).Decode(&file)
		if err != nil {
			return err
		}

		// set content type and length
		ctx.ResponseWriter.Header().Set("Content-Type", file.Type)
		ctx.ResponseWriter.Header().Set("Content-Length", strconv.FormatInt(file.Length, 10))

		// download file
		err = s.Service.Download(ctx, file.Handle, ctx.ResponseWriter)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *Storage) CleanupTask(periodicity, cleanAge time.Duration) *axe.Task {
	return &axe.Task{
		Name: "blaze.CleanupTask",
		Handler: func(ctx *axe.Context) error {
			return s.Cleanup(ctx, cleanAge)
		},
		Workers:      1,
		Periodically: periodicity,
		PeriodicJob: axe.Blueprint{
			Label: "periodic",
		},
	}
}

func (s *Storage) Cleanup(ctx context.Context, cleanAge time.Duration) error {
	// check clean age
	if cleanAge == 0 {
		cleanAge = time.Hour
	}

	// get cursor for old and released files
	csr, err := s.Store.C(&File{}).Find(ctx, bson.M{
		"$or": []bson.M{
			{
				coal.F(&File{}, "State"): bson.M{
					"$in": bson.A{Uploading, Uploaded},
				},
				coal.F(&File{}, "Updated"): bson.M{
					"$lt": time.Now().Add(-cleanAge),
				},
			},
			{
				coal.F(&File{}, "State"): bson.M{
					"$in": bson.A{Released, Deleting},
				},
			},
		},
	})
	if err != nil {
		return err
	}
	defer csr.Close(ctx)

	// iterate over cursor
	for csr.Next(ctx) {
		// decode file
		var file File
		err = csr.Decode(&file)
		if err != nil {
			return err
		}

		// flag file as deleted if not already
		if file.State != Deleting {
			res, err := s.Store.C(&File{}).UpdateOne(ctx, bson.M{
				"_id":                    file.ID,
				coal.F(&File{}, "State"): file.State,
			}, bson.M{
				coal.F(&File{}, "State"):   Deleting,
				coal.F(&File{}, "Updated"): time.Now(),
			})
			if err != nil {
				return err
			}

			// continue if file has been claimed
			if res.ModifiedCount == 0 {
				continue
			}
		}

		// delete upload
		deleted, err := s.Service.Delete(ctx, file.Handle)
		if err != nil {
			return err
		}

		// delete file if upload is deleted
		if deleted {
			_, err = s.Store.C(&File{}).DeleteOne(ctx, bson.M{
				"_id": file.ID(),
			})
			if err != nil {
				return err
			}
		}
	}

	// closer cursor
	_ = csr.Close(ctx)

	// cleanup service
	err = s.Service.Cleanup(ctx)
	if err != nil {
		return err
	}

	return nil
}
