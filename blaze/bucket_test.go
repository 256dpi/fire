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

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

func TestBucketUpload(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(service, "default", true)

		key, file, err := bucket.Upload(nil, strings.Repeat("x", 512), "application/octet-stream", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.Error(t, err)
		assert.Empty(t, key)
		assert.Nil(t, file)
		assert.Equal(t, "file name too long", err.Error())

		key, file, err = bucket.Upload(nil, "data.bin", "", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, key)
		assert.Equal(t, &File{
			Base:    file.Base,
			State:   Uploaded,
			Updated: file.Updated,
			Name:    "data.bin",
			Type:    "application/octet-stream",
			Size:    12,
			Service: "default",
			Handle:  Handle{"id": "1"},
		}, file)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{file}, files)

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)
	})
}

func TestBucketUploadSizeMismatch(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(service, "default", true)

		key, file, err := bucket.Upload(nil, "data.bin", "", 16, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.Error(t, err)
		assert.Empty(t, key)
		assert.Nil(t, file)
		assert.Equal(t, "size mismatch", err.Error())

		key, file, err = bucket.Upload(nil, "data.bin", "", 8, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.Error(t, err)
		assert.Empty(t, key)
		assert.Nil(t, file)
		assert.Equal(t, "size mismatch", err.Error())

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{
			{
				Base:    files[0].Base,
				State:   Uploading,
				Updated: files[0].Updated,
				Name:    "data.bin",
				Type:    "application/octet-stream",
				Size:    16,
				Service: "default",
				Handle:  Handle{"id": "1"},
			},
			{
				Base:    files[1].Base,
				State:   Uploading,
				Updated: files[1].Updated,
				Name:    "data.bin",
				Type:    "application/octet-stream",
				Size:    8,
				Service: "default",
				Handle:  Handle{"id": "2"},
			},
		}, files)

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
			"2": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)
	})
}

func TestBucketUploadAction(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(service, "default", true)

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("Content-Length", "12")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{
			{
				Base:    files[0].Base,
				State:   Uploaded,
				Updated: files[0].Updated,
				Name:    "",
				Type:    "application/octet-stream",
				Size:    12,
				Service: "default",
				Handle:  Handle{"id": "1"},
			},
		}, files)

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)
	})
}

func TestBucketUploadActionExtended(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(service, "default", true)

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Disposition", "foo; filename=data.bin")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "expected attachment content disposition", res.Body.String())

		req.Header.Set("Content-Disposition", "attachment; filename=script.js")
		req.Header.Set("Content-Length", "12")

		res, err = tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{
			{
				Base:    files[0].Base,
				State:   Uploaded,
				Updated: files[0].Updated,
				Name:    "script.js",
				Type:    "application/javascript",
				Size:    12,
				Service: "default",
				Handle:  Handle{"id": "1"},
			},
		}, files)

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "application/javascript",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)
	})
}

func TestBucketUploadActionInvalidContentType(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		body := strings.NewReader("Hello World!")
		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "- _ -")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "invalid content type", res.Body.String())

		assert.Equal(t, 0, tester.Count(&File{}))
	})
}

func TestBucketUploadActionInvalidContentLength(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		body := strings.NewReader("Hello World!")
		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "text/css")
		req.Header.Set("Content-Length", "foo")

		res, err := tester.RunAction(&fire.Context{
			Operation:   fire.CollectionAction,
			HTTPRequest: req,
		}, bucket.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "invalid content length", res.Body.String())

		assert.Equal(t, 0, tester.Count(&File{}))
	})
}

func TestBucketUploadActionLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

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

func TestBucketUploadActionMultipart(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(service, "default", true)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"`},
			"Content-Type":        []string{"text/css"},
			"Content-Length":      []string{"18"},
		})
		assert.NoError(t, err)

		_, err = part.Write([]byte("h1 { color: red; }"))
		assert.NoError(t, err)

		part, err = writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file2"; filename="script.js"`},
			"Content-Type":        []string{"text/javascript"},
			"Content-Length":      []string{"27"},
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

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{
			{
				Base:    files[0].Base,
				State:   Uploaded,
				Updated: files[0].Updated,
				Name:    "",
				Type:    "text/css",
				Size:    18,
				Service: "default",
				Handle:  Handle{"id": "1"},
			},
			{
				Base:    files[1].Base,
				State:   Uploaded,
				Updated: files[1].Updated,
				Name:    "script.js",
				Type:    "text/javascript",
				Size:    27,
				Service: "default",
				Handle:  Handle{"id": "2"},
			},
		}, files)

		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "text/css",
				Bytes: []byte("h1 { color: red; }"),
			},
			"2": {
				Type:  "text/javascript",
				Bytes: []byte("console.log('Hello World!);"),
			},
		}, service.Blobs)
	})
}

func TestBucketUploadActionMultipartInvalidContentType(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"`},
			"Content-Type":        []string{"- _ -"},
			"Content-Length":      []string{"27"},
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
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "invalid content type", res.Body.String())
	})
}

func TestBucketUploadActionMultipartInvalidContentLength(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"`},
			"Content-Type":        []string{"text/css"},
			"Content-Length":      []string{"foo"},
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
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "invalid content length", res.Body.String())
	})
}

func TestBucketUploadActionMultipartLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"; filename="style.css"`},
			"Content-Type":        []string{"text/css"},
			"Content-Length":      []string{"27"},
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

func TestBucketGetViewKey(t *testing.T) {
	bucket := NewBucket(nil, testNotary)

	id := coal.New()

	viewKey1, err := bucket.GetViewKey(nil, id)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	viewKey2, err := bucket.GetViewKey(nil, id)
	assert.NoError(t, err)
	assert.Equal(t, viewKey1, viewKey2)
}

func TestBucketClaimDecorateReleaseRequired(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		/* upload */

		key, _, err := bucket.Upload(nil, "", "application/octet-stream", 12, func(upload Upload) (int64, error) {
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

		err = bucket.Decorate(nil, &model.RequiredFile)
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
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		/* upload */

		key, _, err := bucket.Upload(nil, "", "application/octet-stream", 12, func(upload Upload) (int64, error) {
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

		err = bucket.Decorate(nil, model.OptionalFile)
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
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

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
			Type:    "image/png",
			Size:    42,
			Service: "default",
			Handle: Handle{
				"foo": "bar",
			},
		}).(*File)

		claimKey, err := bucket.notary.Issue(nil, &ClaimKey{
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
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

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
			Type:    "image/png",
			Size:    42,
			Service: "default",
			Handle: Handle{
				"foo": "bar",
			},
		}).(*File)

		claimKey1, err := bucket.notary.Issue(nil, &ClaimKey{
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
			Type:    "image/png",
			Size:    42,
			Service: "default",
			Handle: Handle{
				"foo": "bar",
			},
		}).(*File)

		claimKey2, err := bucket.notary.Issue(nil, &ClaimKey{
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
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

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
			Type:    "image/png",
			Size:    42,
			Service: "default",
			Handle: Handle{
				"foo": "bar",
			},
		}).(*File)

		claimKey1, err := bucket.notary.Issue(nil, &ClaimKey{
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
			Type:    "image/png",
			Size:    42,
			Service: "default",
			Handle: Handle{
				"foo": "bar",
			},
		}).(*File)

		claimKey2, err := bucket.notary.Issue(nil, &ClaimKey{
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
			Type:    "image/png",
			Size:    42,
			Service: "default",
			Handle: Handle{
				"foo": "bar",
			},
		}).(*File)

		claimKey3, err := bucket.notary.Issue(nil, &ClaimKey{
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
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

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
		err = bucket.notary.Verify(nil, &viewKey1, model.RequiredFile.ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file1, viewKey1.File)

		var viewKey2 ViewKey
		err = bucket.notary.Verify(nil, &viewKey2, model.OptionalFile.ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file2, viewKey2.File)

		var viewKey3 ViewKey
		err = bucket.notary.Verify(nil, &viewKey3, model.MultipleFiles[0].ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file3, viewKey3.File)

		var viewKey4 ViewKey
		err = bucket.notary.Verify(nil, &viewKey4, model.MultipleFiles[1].ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file4, viewKey4.File)
	})
}

func TestBucketDownload(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		_, file, err := bucket.Upload(nil, "file", "foo/bar", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Claimed
		file.Binding = "foo"
		file.Owner = stick.P(coal.New())
		tester.Replace(file)

		key, err := bucket.GetViewKey(nil, file.ID())
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
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

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

		_, file, err := bucket.Upload(nil, "file", "foo/bar", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Claimed
		file.Updated = time.Date(2020, 5, 25, 12, 0, 0, 0, time.UTC)
		file.Binding = "test-req"
		file.Owner = stick.P(coal.New())
		tester.Replace(file)

		key, err := bucket.GetViewKey(nil, file.ID())
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
			"Cache-Control":       []string{"public, max-age=31536000"},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
			"Etag":                []string{`"v1-` + file.ID().Hex() + `"`},
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
			"Cache-Control":       []string{"public, max-age=31536000"},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
			"Etag":                []string{`"v1-` + file.ID().Hex() + `"`},
		}, rec.Header())
		assert.Equal(t, "Hello World!", rec.Body.String())
	})
}

func TestBucketDownloadActionStream(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(NewMemory(), "default", true)

		_, file, err := bucket.Upload(nil, "file", "foo/bar", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		file.State = Claimed
		file.Updated = time.Date(2020, 5, 25, 12, 0, 0, 0, time.UTC)
		file.Binding = "test-req"
		file.Owner = stick.P(coal.New())
		tester.Replace(file)

		key, err := bucket.GetViewKey(nil, file.ID())
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
			"Cache-Control":       []string{"public, max-age=31536000"},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
			"Etag":                []string{`"v1-` + file.ID().Hex() + `"`},
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
			"Cache-Control":       []string{"public, max-age=31536000"},
			"Last-Modified":       []string{"Mon, 25 May 2020 12:00:00 GMT"},
			"Etag":                []string{`"v1-` + file.ID().Hex() + `"`},
		}, rec.Header())
		assert.Equal(t, "foo/bar", rec.Header().Get("Content-Type"))
		assert.Equal(t, "", rec.Body.String())
	})
}

func TestBucketCleanup(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		svc := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(svc, "default", true)

		for _, state := range []State{Uploading, Uploaded, Claimed, Released, Deleting} {
			_, file, err := bucket.Upload(nil, "file", "foo/bar", 12, func(upload Upload) (int64, error) {
				return UploadFrom(upload, strings.NewReader("Hello World!"))
			})
			assert.NoError(t, err)
			assert.NotNil(t, file)

			file.State = state
			if state == Claimed {
				file.Binding = "foo"
				file.Owner = stick.P(coal.New())
			}
			tester.Replace(file)
		}

		assert.Equal(t, 5, tester.Count(&File{}))
		assert.Len(t, svc.Blobs, 5)

		/* cleanup */

		queue := axe.NewQueue(axe.Options{
			Store:    tester.Store,
			Reporter: xo.Panic,
		})

		task := bucket.CleanupTask(1, 5)

		notify := make(chan *axe.Context, 1)
		task.Notifier = func(ctx *axe.Context, cancelled bool, reason string) error {
			notify <- ctx
			return nil
		}

		queue.Add(task)
		<-queue.Run()
		defer queue.Close()

		/* first iteration */

		ctx := <-notify
		assert.Equal(t, "scan", ctx.Job.GetBase().Label)

		for i := 0; i < 4; i++ {
			<-notify
		}

		/* second iteration */

		task.PeriodicJob.Job.GetBase().DocID = coal.New()
		_, err := queue.Enqueue(nil, task.PeriodicJob.Job, 0, 0)
		assert.NoError(t, err)

		ctx = <-notify
		assert.Equal(t, "scan", ctx.Job.GetBase().Label)

		for i := 0; i < 4; i++ {
			<-notify
		}

		/* third iteration */

		task.PeriodicJob.Job.GetBase().DocID = coal.New()
		_, err = queue.Enqueue(nil, task.PeriodicJob.Job, 0, 0)
		assert.NoError(t, err)

		ctx = <-notify
		assert.Equal(t, "scan", ctx.Job.GetBase().Label)

		for i := 0; i < 3; i++ {
			<-notify
		}

		/* done */

		assert.Equal(t, 1, tester.Count(&File{}))
		assert.Len(t, svc.Blobs, 1)
	})
}

func TestBucketMultiService(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		svc1 := NewMemory()
		svc2 := NewMemory()
		svc3 := NewMemory()

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(svc1, "svc1", true)
		bucket.Use(svc2, "svc2", true)
		bucket.Use(svc3, "svc3", false)

		var files []*File
		for i := 0; i < 20; i++ {
			claimKey, file, err := bucket.Upload(nil, "file", "foo/bar", 12, func(upload Upload) (int64, error) {
				return UploadFrom(upload, strings.NewReader("Hello World!"))
			})
			assert.NoError(t, err)
			assert.NotNil(t, file)
			files = append(files, file)

			_, err = bucket.ClaimFile(nil, claimKey, "test-req", coal.New())
			assert.NoError(t, err)
		}

		assert.NotZero(t, len(svc1.Blobs))
		assert.NotZero(t, len(svc2.Blobs))
		assert.Zero(t, len(svc3.Blobs))

		for i := 0; i < 20; i++ {
			dl, _, err := bucket.DownloadFile(nil, files[i].ID())
			assert.NoError(t, err)

			var buf bytes.Buffer
			err = DownloadTo(dl, &buf)
			assert.NoError(t, err)
			assert.Equal(t, "Hello World!", buf.String())
		}
	})
}

func TestBucketMigration(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		svc1 := NewMemory()
		svc2 := NewMemory()

		/* upload */

		bucket := NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(svc1, "svc1", true)

		claimKey, file, err := bucket.Upload(nil, "file", "foo/bar", 12, func(upload Upload) (int64, error) {
			return UploadFrom(upload, strings.NewReader("Hello World!"))
		})
		assert.NoError(t, err)
		assert.NotNil(t, file)

		owner := coal.New()

		file, err = bucket.ClaimFile(nil, claimKey, "test-req", owner)
		assert.NoError(t, err)
		assert.NotNil(t, file)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{
			{
				Base:    files[0].Base,
				State:   Claimed,
				Updated: files[0].Updated,
				Name:    "file",
				Type:    "foo/bar",
				Size:    12,
				Service: "svc1",
				Handle:  Handle{"id": "1"},
				Binding: "test-req",
				Owner:   files[0].Owner,
			},
		}, files)
		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "foo/bar",
				Bytes: []byte("Hello World!"),
			},
		}, svc1.Blobs)
		assert.Empty(t, svc2.Blobs)

		/* migrate */

		bucket = NewBucket(tester.Store, testNotary, bindings.All()...)
		bucket.Use(svc1, "svc1", false)
		bucket.Use(svc2, "svc2", true)

		queue := axe.NewQueue(axe.Options{
			Store:    tester.Store,
			Reporter: xo.Panic,
		})

		task := bucket.MigrateTask([]string{"svc1"}, 5)

		notify := make(chan *axe.Context, 1)
		task.Notifier = func(ctx *axe.Context, cancelled bool, reason string) error {
			notify <- ctx
			return nil
		}

		queue.Add(task)
		<-queue.Run()
		defer queue.Close()

		ctx := <-notify
		assert.Equal(t, "scan", ctx.Job.GetBase().Label)

		ctx = <-notify
		assert.Equal(t, file.ID().Hex(), ctx.Job.GetBase().Label)

		files = *tester.FindAll(&File{}).(*[]*File)
		assert.Equal(t, []*File{
			{
				Base:    files[0].Base,
				State:   Claimed,
				Updated: files[0].Updated,
				Name:    "file",
				Type:    "foo/bar",
				Size:    12,
				Service: "svc2",
				Handle:  Handle{"id": "1"},
				Binding: "test-req",
				Owner:   stick.P(owner),
			},
			{
				Base:    files[1].Base,
				State:   Released,
				Updated: files[1].Updated,
				Name:    "file",
				Type:    "foo/bar",
				Size:    12,
				Service: "svc1",
				Handle:  Handle{"id": "1"},
				Binding: "",
				Owner:   nil,
			},
		}, files)
		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "foo/bar",
				Bytes: []byte("Hello World!"),
			},
		}, svc1.Blobs)
		assert.Equal(t, map[string]*MemoryBlob{
			"1": {
				Type:  "foo/bar",
				Bytes: []byte("Hello World!"),
			},
		}, svc2.Blobs)
	})
}
