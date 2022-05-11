package blaze

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

func TestBucketUpload(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()
		bucket := NewBucket(tester.Store, testNotary, service, register)

		key, file, err := bucket.Upload(nil, "data.bin", "application/octet-stream", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)
		assert.Equal(t, Uploaded, file.State)
		assert.Equal(t, "application/octet-stream", file.Type)
		assert.Equal(t, int64(12), file.Size)
		assert.Equal(t, Handle{"id": "1"}, file.Handle)
		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Name:  "data.bin",
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Len(t, files, 1)
		assert.Equal(t, Uploaded, files[0].State)
		assert.Equal(t, "application/octet-stream", files[0].Type)
		assert.Equal(t, int64(12), files[0].Size)
		assert.Equal(t, Handle{"id": "1"}, files[0].Handle)
	})
}

func TestBucketUploadAction(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()
		bucket := NewBucket(tester.Store, testNotary, service, register)

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "application/octet-stream")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())
		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Len(t, files, 1)
		assert.Equal(t, Uploaded, files[0].State)
		assert.Equal(t, "application/octet-stream", files[0].Type)
		assert.Equal(t, int64(12), files[0].Size)
		assert.Equal(t, Handle{"id": "1"}, files[0].Handle)
	})
}

func TestBucketUploadActionExtended(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()
		bucket := NewBucket(tester.Store, testNotary, service, register)

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Disposition", "foo; filename=data.bin")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.Error(t, err)
		assert.Equal(t, http.StatusInternalServerError, res.Code)
		assert.Equal(t, "expected attachment content disposition", err.Error())

		req.Header.Set("Content-Disposition", "attachment; filename=script.js")

		res, err = tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())
		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Name:  "script.js",
				Type:  "application/javascript",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Len(t, files, 1)
		assert.Equal(t, Uploaded, files[0].State)
		assert.Equal(t, "application/javascript", files[0].Type)
		assert.Equal(t, int64(12), files[0].Size)
		assert.Equal(t, Handle{"id": "1"}, files[0].Handle)
	})
}

func TestBucketUploadActionInvalidContentType(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		body := strings.NewReader("Hello World!")
		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "- _ -")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "", res.Body.String())

		assert.Equal(t, 0, tester.Count(&File{}))
	})
}

func TestBucketUploadActionLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "application/octet-stream")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(1))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		assert.Equal(t, "", res.Body.String())
	})
}

func TestBucketUploadActionFormFiles(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()
		bucket := NewBucket(tester.Store, testNotary, service, register)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file1", "")
		assert.NoError(t, err)

		_, err = part.Write([]byte("Hello World 1!"))
		assert.NoError(t, err)

		part, err = writer.CreateFormFile("file2", "data2.bin")
		assert.NoError(t, err)

		_, err = part.Write([]byte("Hello World 2!"))
		assert.NoError(t, err)

		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World 1!"),
			},
			"2": {
				Name:  "data2.bin",
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World 2!"),
			},
		}, service.Blobs)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Len(t, files, 2)
		assert.Equal(t, Uploaded, files[0].State)
		assert.Equal(t, "application/octet-stream", files[0].Type)
		assert.Equal(t, int64(14), files[0].Size)
		assert.Equal(t, Handle{"id": "1"}, files[0].Handle)
		assert.Equal(t, Uploaded, files[1].State)
		assert.Equal(t, "application/octet-stream", files[1].Type)
		assert.Equal(t, int64(14), files[1].Size)
		assert.Equal(t, Handle{"id": "2"}, files[1].Handle)
	})
}

func TestBucketUploadActionFormFilesLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file1", "data1.bin")
		assert.NoError(t, err)

		_, err = part.Write([]byte("Hello World 1!"))
		assert.NoError(t, err)

		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(1))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		assert.Equal(t, "", res.Body.String())
	})
}

func TestBucketUploadActionMultipart(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()
		bucket := NewBucket(tester.Store, testNotary, service, register)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"`},
			"Content-Type":        []string{"text/css"},
		})
		assert.NoError(t, err)

		_, err = part.Write([]byte("h1 { color: red; }"))
		assert.NoError(t, err)

		part, err = writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file2"; filename="script.js"`},
			"Content-Type":        []string{"text/javascript"},
		})
		assert.NoError(t, err)

		_, err = part.Write([]byte("console.log('Hello World!);"))
		assert.NoError(t, err)

		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "text/css",
				Bytes: []byte("h1 { color: red; }"),
			},
			"2": {
				Name:  "script.js",
				Type:  "text/javascript",
				Bytes: []byte("console.log('Hello World!);"),
			},
		}, service.Blobs)
	})
}

func TestBucketUploadActionMultipartLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"; filename="style.css"`},
			"Content-Type":        []string{"text/css"},
		})
		assert.NoError(t, err)

		_, err = part.Write([]byte("console.log('Hello World!);"))
		assert.NoError(t, err)

		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(1))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		assert.Equal(t, "", res.Body.String())
	})
}

func TestBucketClaimDecorateReleaseRequired(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		/* upload */

		key, _, err := bucket.Upload(nil, "", "application/octet-stream", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		model := &testModel{
			Base: coal.B(),
		}

		/* claim without key */

		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Claim(ctx, model, "RequiredFile")
		})
		assert.Error(t, err)
		assert.Equal(t, "missing claim key", err.Error())

		/* claim with key */

		model.RequiredFile.ClaimKey = key
		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Claim(ctx, model, "RequiredFile")
		})
		assert.NoError(t, err)

		/* decorate */

		err = bucket.Decorate(&model.RequiredFile)
		assert.NoError(t, err)

		/* download */

		download, _, err := bucket.Download(nil, model.RequiredFile.ViewKey)
		assert.NoError(t, err)

		var buffer bytes.Buffer
		err = DownloadTo(download, &buffer)
		assert.NoError(t, err)
		assert.Equal(t, "Hello World!", buffer.String())

		/* release */

		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Release(ctx, model, "RequiredFile")
		})
		assert.NoError(t, err)

		/* release again */

		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Release(ctx, model, "RequiredFile")
		})
		assert.Error(t, err)
		assert.Equal(t, "invalid file id", err.Error())
	})
}

func TestBucketClaimDecorateReleaseOptional(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		/* upload */

		key, _, err := bucket.Upload(nil, "", "application/octet-stream", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		model := &testModel{
			Base: coal.B(),
		}

		/* claim without link */

		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Claim(ctx, model, "OptionalFile")
		})
		assert.Error(t, err)
		assert.Equal(t, "missing link", err.Error())

		/* claim without key */

		model.OptionalFile = &Link{}
		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Claim(ctx, model, "OptionalFile")
		})
		assert.Error(t, err)
		assert.Equal(t, "missing claim key", err.Error())

		/* claim with key */

		model.OptionalFile.ClaimKey = key
		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Claim(ctx, model, "OptionalFile")
		})
		assert.NoError(t, err)

		/* decorate */

		err = bucket.Decorate(model.OptionalFile)
		assert.NoError(t, err)

		/* download */

		download, _, err := bucket.Download(nil, model.OptionalFile.ViewKey)
		assert.NoError(t, err)

		var buffer bytes.Buffer
		err = DownloadTo(download, &buffer)
		assert.NoError(t, err)
		assert.Equal(t, "Hello World!", buffer.String())

		/* release */

		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Release(ctx, model, "OptionalFile")
		})
		assert.NoError(t, err)

		/* release again */

		err = tester.Store.T(nil, false, func(ctx context.Context) error {
			return bucket.Release(ctx, model, "OptionalFile")
		})
		assert.Error(t, err)
		assert.Equal(t, "missing link", err.Error())
	})
}

func TestBucketModifierRequired(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		modifier := bucket.Modifier()

		/* missing */

		model := &testModel{
			Base: coal.B(),
		}
		err := tester.RunCallback(&fire.Context{
			Operation: fire.Create,
			Model:     model,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.Error(t, err)
		assert.Equal(t, "RequiredFile: missing claim key", err.Error())

		/* required */

		file := tester.Insert(&File{
			State:   Uploaded,
			Updated: time.Now(),
			Size:    42,
			Handle: Handle{
				"foo": "bar",
			},
			Type: "image/png",
		}).(*File)

		claimKey, err := bucket.notary.Issue(&ClaimKey{
			File: file.ID(),
			Size: file.Size,
			Type: file.Type,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey)

		model.RequiredFile.ClaimKey = claimKey

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Create,
			Model:     model,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)
		assert.Equal(t, file.ID(), model.RequiredFile.File)

		file = tester.Fetch(&File{}, file.ID()).(*File)
		assert.Equal(t, Claimed, file.State)
	})
}

func TestBucketModifierOptional(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		modifier := bucket.Modifier()

		model := &testModel{
			Base: coal.B(),
			RequiredFile: Link{
				File: coal.New(),
			},
		}

		/* add */

		file1 := tester.Insert(&File{
			State:   Uploaded,
			Updated: time.Now(),
			Size:    42,
			Handle: Handle{
				"foo": "bar",
			},
			Type: "image/png",
		}).(*File)

		claimKey1, err := bucket.notary.Issue(&ClaimKey{
			File: file1.ID(),
			Size: file1.Size,
			Type: file1.Type,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey1)

		model.OptionalFile = &Link{ClaimKey: claimKey1}

		original := &testModel{
			Base:         model.Base,
			RequiredFile: model.RequiredFile,
		}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)
		assert.Equal(t, file1.ID(), model.OptionalFile.File)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Claimed, file1.State)

		/* update */

		file2 := tester.Insert(&File{
			State:   Uploaded,
			Updated: time.Now(),
			Size:    42,
			Handle: Handle{
				"foo": "bar",
			},
			Type: "image/png",
		}).(*File)

		claimKey2, err := bucket.notary.Issue(&ClaimKey{
			File: file2.ID(),
			Size: file2.Size,
			Type: file2.Type,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey2)

		original.OptionalFile = model.OptionalFile
		model.OptionalFile = &Link{ClaimKey: claimKey2}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)
		assert.Equal(t, file2.ID(), model.OptionalFile.File)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Released, file1.State)

		file2 = tester.Fetch(&File{}, file2.ID()).(*File)
		assert.Equal(t, Claimed, file2.State)

		/* remove */

		original.OptionalFile = model.OptionalFile
		model.OptionalFile = nil

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)

		file2 = tester.Fetch(&File{}, file2.ID()).(*File)
		assert.Equal(t, Released, file2.State)
	})
}

func TestBucketModifierMultiple(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		modifier := bucket.Modifier()

		model := &testModel{
			Base: coal.B(),
			RequiredFile: Link{
				File: coal.New(),
			},
		}

		/* add first */

		file1 := tester.Insert(&File{
			State:   Uploaded,
			Updated: time.Now(),
			Size:    42,
			Handle: Handle{
				"foo": "bar",
			},
			Type: "image/png",
		}).(*File)

		claimKey1, err := bucket.notary.Issue(&ClaimKey{
			File: file1.ID(),
			Size: file1.Size,
			Type: file1.Type,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey1)

		model.MultipleFiles = Links{
			{Ref: "1", ClaimKey: claimKey1},
		}

		original := &testModel{
			Base:         model.Base,
			RequiredFile: model.RequiredFile,
		}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)
		assert.Equal(t, file1.ID(), model.MultipleFiles[0].File)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Claimed, file1.State)

		/* add second */

		file2 := tester.Insert(&File{
			State:   Uploaded,
			Updated: time.Now(),
			Size:    42,
			Handle: Handle{
				"foo": "bar",
			},
			Type: "image/png",
		}).(*File)

		claimKey2, err := bucket.notary.Issue(&ClaimKey{
			File: file2.ID(),
			Size: file2.Size,
			Type: file2.Type,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey2)

		original.MultipleFiles = model.MultipleFiles
		model.MultipleFiles = Links{
			model.MultipleFiles[0],
			{Ref: "2", ClaimKey: claimKey2},
		}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)
		assert.Equal(t, file2.ID(), model.MultipleFiles[1].File)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Claimed, file1.State)

		file2 = tester.Fetch(&File{}, file2.ID()).(*File)
		assert.Equal(t, Claimed, file2.State)

		/* update first */

		file3 := tester.Insert(&File{
			State:   Uploaded,
			Updated: time.Now(),
			Size:    42,
			Handle: Handle{
				"foo": "bar",
			},
			Type: "image/png",
		}).(*File)

		claimKey3, err := bucket.notary.Issue(&ClaimKey{
			File: file3.ID(),
			Size: file3.Size,
			Type: file3.Type,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey3)

		original.MultipleFiles = model.MultipleFiles
		model.MultipleFiles = Links{
			{Ref: "1", ClaimKey: claimKey3},
			model.MultipleFiles[1],
		}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)
		assert.Equal(t, file2.ID(), model.MultipleFiles[1].File)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Released, file1.State)

		file2 = tester.Fetch(&File{}, file2.ID()).(*File)
		assert.Equal(t, Claimed, file2.State)

		file3 = tester.Fetch(&File{}, file3.ID()).(*File)
		assert.Equal(t, Claimed, file3.State)

		/* remove second */

		original.MultipleFiles = model.MultipleFiles
		model.MultipleFiles = Links{
			model.MultipleFiles[0],
		}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Released, file1.State)

		file2 = tester.Fetch(&File{}, file2.ID()).(*File)
		assert.Equal(t, Released, file2.State)

		file3 = tester.Fetch(&File{}, file3.ID()).(*File)
		assert.Equal(t, Claimed, file3.State)

		/* remove first */

		original.MultipleFiles = model.MultipleFiles
		model.MultipleFiles = Links{}

		err = tester.RunCallback(&fire.Context{
			Operation: fire.Update,
			Model:     model,
			Original:  original,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, modifier)
		assert.NoError(t, err)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Released, file1.State)

		file2 = tester.Fetch(&File{}, file2.ID()).(*File)
		assert.Equal(t, Released, file2.State)

		file3 = tester.Fetch(&File{}, file3.ID()).(*File)
		assert.Equal(t, Released, file3.State)
	})
}

func TestBucketDecorator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		decorator := bucket.Decorator()

		file1 := coal.New()
		file2 := coal.New()
		file3 := coal.New()
		file4 := coal.New()

		model := &testModel{
			RequiredFile: Link{
				File:     file1,
				FileName: "file1",
				FileType: "foo/bar",
				FileSize: 42,
			},
			OptionalFile: &Link{
				File:     file2,
				FileName: "file2",
				FileType: "foo/bar",
				FileSize: 42,
			},
			MultipleFiles: Links{
				{
					File:     file3,
					FileName: "file3",
					FileType: "foo/bar",
					FileSize: 42,
				},
				{
					File:     file4,
					FileName: "file4",
					FileType: "foo/bar",
					FileSize: 42,
				},
			},
		}
		err := tester.RunCallback(&fire.Context{
			Operation: fire.Find,
			Model:     model,
			Controller: &fire.Controller{
				Model: &testModel{},
			},
		}, decorator)
		assert.NoError(t, err)
		assert.NotEmpty(t, model.RequiredFile.ViewKey)
		assert.NotEmpty(t, model.OptionalFile.ViewKey)
		assert.NotEmpty(t, model.MultipleFiles[0].ViewKey)
		assert.NotEmpty(t, model.MultipleFiles[1].ViewKey)
		assert.Equal(t, Link{
			Name:     "file1",
			Type:     "foo/bar",
			Size:     42,
			ViewKey:  model.RequiredFile.ViewKey,
			File:     file1,
			FileName: "file1",
			FileType: "foo/bar",
			FileSize: 42,
		}, model.RequiredFile)
		assert.Equal(t, &Link{
			Name:     "file2",
			Type:     "foo/bar",
			Size:     42,
			ViewKey:  model.OptionalFile.ViewKey,
			File:     file2,
			FileName: "file2",
			FileType: "foo/bar",
			FileSize: 42,
		}, model.OptionalFile)
		assert.Equal(t, Links{
			{
				Name:     "file3",
				Type:     "foo/bar",
				Size:     42,
				ViewKey:  model.MultipleFiles[0].ViewKey,
				File:     file3,
				FileName: "file3",
				FileType: "foo/bar",
				FileSize: 42,
			},
			{
				Name:     "file4",
				Type:     "foo/bar",
				Size:     42,
				ViewKey:  model.MultipleFiles[1].ViewKey,
				File:     file4,
				FileName: "file4",
				FileType: "foo/bar",
				FileSize: 42,
			},
		}, model.MultipleFiles)

		var viewKey1 ViewKey
		err = bucket.notary.Verify(&viewKey1, model.RequiredFile.ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file1, viewKey1.File)

		var viewKey2 ViewKey
		err = bucket.notary.Verify(&viewKey2, model.OptionalFile.ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file2, viewKey2.File)

		var viewKey3 ViewKey
		err = bucket.notary.Verify(&viewKey3, model.MultipleFiles[0].ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file3, viewKey3.File)

		var viewKey4 ViewKey
		err = bucket.notary.Verify(&viewKey4, model.MultipleFiles[1].ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file4, viewKey4.File)
	})
}

func TestBucketDownload(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		_, file, err := bucket.Upload(nil, "file", "foo/bar", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Claimed
		file.Binding = "foo"
		file.Owner = coal.P(coal.New())
		tester.Replace(file)

		key, err := bucket.notary.Issue(&ViewKey{
			Base: heat.Base{},
			File: file.ID(),
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		download, file, err := bucket.Download(nil, key)
		assert.NoError(t, err)
		assert.NotEmpty(t, file)

		var buffer bytes.Buffer
		err = DownloadTo(download, &buffer)
		assert.NoError(t, err)
		assert.Equal(t, "Hello World!", buffer.String())
	})
}

func TestBucketDownloadAction(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		action := bucket.DownloadAction()

		/* no key */

		req := httptest.NewRequest("GET", "/foo", nil)
		rec, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, action)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, http.Header{}, rec.Header())
		assert.Equal(t, "", rec.Body.String())

		/* with key */

		_, file, err := bucket.Upload(nil, "file", "foo/bar", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Claimed
		file.Updated = time.Date(2020, 5, 25, 12, 0, 0, 0, time.UTC)
		file.Binding = "test-req"
		file.Owner = coal.P(coal.New())
		tester.Replace(file)

		key, err := bucket.notary.Issue(&ViewKey{
			Base: heat.Base{},
			File: file.ID(),
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		req = httptest.NewRequest("GET", "/foo?key="+key, nil)
		rec, err = tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, action)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, http.Header{
			"Accept-Ranges":       []string{"bytes"},
			"Content-Length":      []string{"12"},
			"Content-Type":        []string{"foo/bar"},
			"Content-Disposition": []string{`inline`},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
		}, rec.Header())
		assert.Equal(t, "Hello World!", rec.Body.String())

		/* attachment */

		req = httptest.NewRequest("GET", "/foo?key="+key+"&dl=1", nil)
		rec, err = tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, action)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, http.Header{
			"Accept-Ranges":       []string{"bytes"},
			"Content-Length":      []string{"12"},
			"Content-Type":        []string{"foo/bar"},
			"Content-Disposition": []string{`attachment; filename="forced"`},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
		}, rec.Header())
		assert.Equal(t, "Hello World!", rec.Body.String())
	})
}

func TestBucketDownloadActionStream(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		_, file, err := bucket.Upload(nil, "file", "foo/bar", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Claimed
		file.Updated = time.Date(2020, 5, 25, 12, 0, 0, 0, time.UTC)
		file.Binding = "test-req"
		file.Owner = coal.P(coal.New())
		tester.Replace(file)

		key, err := bucket.notary.Issue(&ViewKey{
			Base: heat.Base{},
			File: file.ID(),
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)

		action := bucket.DownloadAction()

		req := httptest.NewRequest("HEAD", "/foo?key="+key, nil)
		rec, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, action)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, http.Header{
			"Accept-Ranges":       []string{"bytes"},
			"Content-Length":      []string{"12"},
			"Content-Type":        []string{"foo/bar"},
			"Content-Disposition": []string{`inline`},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
		}, rec.Header())
		assert.Equal(t, "foo/bar", rec.Header().Get("Content-Type"))
		assert.Equal(t, "", rec.Body.String())

		req = httptest.NewRequest("HEAD", "/foo?key="+key, nil)
		req.Header.Set("Range", "bytes=0-5")
		rec, err = tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, action)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusPartialContent, rec.Code)
		assert.Equal(t, http.Header{
			"Accept-Ranges":       []string{"bytes"},
			"Content-Length":      []string{"6"},
			"Content-Type":        []string{"foo/bar"},
			"Content-Disposition": []string{`inline`},
			"Content-Range":       []string{"bytes 0-5/12"},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
		}, rec.Header())
		assert.Equal(t, "foo/bar", rec.Header().Get("Content-Type"))
		assert.Equal(t, "", rec.Body.String())
	})
}

func TestBucketCleanup(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, NewMemory(), register)

		_, file, err := bucket.Upload(nil, "file", "foo/bar", func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Released
		tester.Replace(file)

		time.Sleep(10 * time.Millisecond)

		err = bucket.Cleanup(nil, 1)
		assert.NoError(t, err)
		assert.Equal(t, 0, tester.Count(&File{}))
	})
}
