package blaze

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

// TODO: Test claim keys.

func TestUploadInvalidContentType(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		storage := &Storage{
			Store:   tester.Store,
			Service: NewMemory(),
		}

		body := strings.NewReader("Hello World!")
		req := httptest.NewRequest("POST", "/foo", body)

		res, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, storage.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "", res.Body.String())

		assert.Equal(t, 0, tester.Count(&File{}))
	})
}

func TestUploadRaw(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: service,
		}

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "application/octet-stream")

		res, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, storage.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())
		assert.Equal(t, map[string]Blob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World!"),
			},
		}, service.Blobs)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Len(t, files, 1)
		assert.Equal(t, Uploaded, files[0].State)
		assert.Equal(t, "application/octet-stream", files[0].Type)
		assert.Equal(t, int64(12), files[0].Length)
		assert.Equal(t, Handle{"id": "1"}, files[0].Handle)
	})
}

func TestUploadRawLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: NewMemory(),
		}

		body := strings.NewReader("Hello World!")

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", "application/octet-stream")

		res, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, storage.UploadAction(1))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		assert.Equal(t, "", res.Body.String())
	})
}

func TestUploadFormFiles(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: service,
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file1", "data1.bin")
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
			HTTPRequest: req,
		}, storage.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())

		assert.Equal(t, map[string]Blob{
			"1": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World 1!"),
			},
			"2": {
				Type:  "application/octet-stream",
				Bytes: []byte("Hello World 2!"),
			},
		}, service.Blobs)

		files := *tester.FindAll(&File{}).(*[]*File)
		assert.Len(t, files, 2)
		assert.Equal(t, Uploaded, files[0].State)
		assert.Equal(t, "application/octet-stream", files[0].Type)
		assert.Equal(t, int64(14), files[0].Length)
		assert.Equal(t, Handle{"id": "1"}, files[0].Handle)
		assert.Equal(t, Uploaded, files[1].State)
		assert.Equal(t, "application/octet-stream", files[1].Type)
		assert.Equal(t, int64(14), files[1].Length)
		assert.Equal(t, Handle{"id": "2"}, files[1].Handle)
	})
}

func TestUploadFormFilesLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: NewMemory(),
		}

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
			HTTPRequest: req,
		}, storage.UploadAction(1))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		assert.Equal(t, "", res.Body.String())
	})
}

func TestUploadMultipart(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		service := NewMemory()

		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: service,
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"; filename="style.css"`},
			"Content-Type":        []string{"text/css"},
		})

		_, err = part.Write([]byte("h1 { color: red; }"))
		assert.NoError(t, err)

		part, err = writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file2"; filename="script.js"`},
			"Content-Type":        []string{"text/javascript"},
		})

		_, err = part.Write([]byte("console.log('Hello World!);"))
		assert.NoError(t, err)

		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, storage.UploadAction(0))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.NotEmpty(t, res.Body.String())

		assert.Equal(t, map[string]Blob{
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

func TestUploadMultipartLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: NewMemory(),
		}

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="file1"; filename="style.css"`},
			"Content-Type":        []string{"text/css"},
		})

		_, err = part.Write([]byte("console.log('Hello World!);"))
		assert.NoError(t, err)

		err = writer.Close()
		assert.NoError(t, err)

		req := httptest.NewRequest("POST", "/foo", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		res, err := tester.RunAction(&fire.Context{
			HTTPRequest: req,
		}, storage.UploadAction(1))
		assert.NoError(t, err)
		assert.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		assert.Equal(t, "", res.Body.String())
	})
}

func TestValidator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		storage := &Storage{
			Store:   tester.Store,
			Notary:  testNotary,
			Service: NewMemory(),
		}

		validator := storage.Validator()

		/* missing */

		model := &testModel{}
		err := tester.RunCallback(&fire.Context{
			Operation: fire.Create,
			Model:     model,
			Controller: &fire.Controller{
				Model: coal.Init(&testModel{}),
			},
		}, validator)
		assert.Error(t, err)
		assert.Equal(t, "required-file: missing claim key", err.Error())

		/* required */

		file1 := tester.Save(&File{
			State: Uploaded,
		}).(*File)

		claimKey1, err := storage.Notary.Issue(&ClaimKey{
			File: file1.ID(),
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, claimKey1)

		model = &testModel{
			RequiredFile: Link{
				ClaimKey: claimKey1,
			},
		}
		err = tester.RunCallback(&fire.Context{
			Operation: fire.Create,
			Model:     model,
			Controller: &fire.Controller{
				Model: coal.Init(&testModel{}),
			},
		}, validator)
		assert.NoError(t, err)
		assert.Equal(t, file1.ID(), *model.RequiredFile.File)

		file1 = tester.Fetch(&File{}, file1.ID()).(*File)
		assert.Equal(t, Claimed, file1.State)
	})
}

func TestDecorator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		storage := &Storage{
			Notary: testNotary,
		}

		decorator := storage.Decorator()

		file1 := coal.New()
		file2 := coal.New()

		model := &testModel{
			RequiredFile: Link{
				File: coal.P(file1),
			},
			OptionalFile: &Link{
				File: coal.P(file2),
			},
		}
		err := tester.RunCallback(&fire.Context{
			Operation: fire.Find,
			Model:     model,
			Controller: &fire.Controller{
				Model: coal.Init(&testModel{}),
			},
		}, decorator)
		assert.NoError(t, err)
		assert.NotEmpty(t, model.RequiredFile.ViewKey)
		assert.NotEmpty(t, model.OptionalFile.ViewKey)

		var viewKey1 ViewKey
		err = storage.Notary.Verify(&viewKey1, model.RequiredFile.ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file1, viewKey1.File)

		var viewKey2 ViewKey
		err = storage.Notary.Verify(&viewKey2, model.OptionalFile.ViewKey)
		assert.NoError(t, err)
		assert.Equal(t, file2, viewKey2.File)
	})
}
