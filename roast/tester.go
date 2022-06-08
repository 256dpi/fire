package roast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

// TODO: Support filters and sorters.
// TODO: Support pagination.

// AccessDenied is the raw access denied error value.
var AccessDenied = fire.ErrAccessDenied.Self().(*xo.Err).Err

// ResourceNotFound is the raw resource not found error value.
var ResourceNotFound = fire.ErrResourceNotFound.Self().(*xo.Err).Err

// Config provides configuration of a tester.
type Config struct {
	Store            *coal.Store
	Models           []coal.Model
	Handler          http.Handler
	DataNamespace    string
	AuthNamespace    string
	TokenEndpoint    string
	UploadEndpoint   string
	DownloadEndpoint string
	Authorizer       func(req *http.Request)
	Debug            bool
}

// Result is returned by the tester.
type Result struct {
	Error    error
	Model    coal.Model
	Models   []coal.Model
	Document *jsonapi.Document
}

// Tester provides a high-level unit test facility.
type Tester struct {
	*fire.Tester
	RawClient   *http.Client
	DataClient  *fire.Client
	AuthClient  *oauth2.Client
	AuthToken   string
	UploadURL   string
	DownloadURL string
}

// NewTester will create and return a new tester.
func NewTester(config Config) *Tester {
	// prepare tester
	tester := &Tester{
		Tester: fire.NewTester(config.Store, config.Models...),
	}

	// set prefix
	tester.Prefix = config.DataNamespace

	// set handler
	tester.Tester.Handler = config.Handler

	// prepare http client
	tester.RawClient = &http.Client{
		Transport: serve.Local(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.Debug {
				fmt.Println(r.Method, r.URL.String())
			}
			tester.Tester.Handler.ServeHTTP(w, r)
			if config.Debug {
				rec := w.(*httptest.ResponseRecorder)
				fmt.Println(rec.Code, rec.Body.String())
			}
		})),
	}

	// get authorizer
	if config.Authorizer == nil {
		config.Authorizer = func(req *http.Request) {
			if tester.AuthToken != "" {
				req.Header.Set("Authorization", "Bearer "+tester.AuthToken)
			}
		}
	}

	// set data client
	tester.DataClient = fire.NewClient(jsonapi.NewClientWithClient(jsonapi.ClientConfig{
		BaseURI:       "/" + strings.Trim(config.DataNamespace, "/"),
		Authorizer:    config.Authorizer,
		ResponseLimit: serve.MustByteSize("8M"),
	}, tester.RawClient))

	// set auth client
	tester.AuthClient = oauth2.NewClientWithClient(oauth2.ClientConfig{
		BaseURI:       "/" + strings.Trim(config.AuthNamespace, "/"),
		TokenEndpoint: "/" + strings.Trim(config.TokenEndpoint, "/"),
	}, tester.RawClient)

	// set upload URL
	tester.UploadURL = "/" + strings.Trim(config.DataNamespace, "/") + "/" + strings.Trim(config.UploadEndpoint, "/")
	tester.DownloadURL = "/" + strings.Trim(config.DataNamespace, "/") + "/" + strings.Trim(config.DownloadEndpoint, "/")

	return tester
}

// Authenticate will request an access token using the provided credentials.
func (t *Tester) Authenticate(clientID, username, password string, scope ...string) {
	// perform authentication
	res, err := t.AuthClient.Authenticate(oauth2.TokenRequest{
		GrantType: oauth2.PasswordGrantType,
		Scope:     scope,
		ClientID:  clientID,
		Username:  username,
		Password:  password,
	})
	if err != nil {
		panic(err)
	}

	// set auth token
	t.AuthToken = res.AccessToken
}

// Invalidate will clear the current authentication.
func (t *Tester) Invalidate() {
	// clear auth token
	t.AuthToken = ""
}

// List will list the provided model and validate the response if requested.
func (t *Tester) List(tt *testing.T, model coal.Model, response []coal.Model) Result {
	// list resources
	res, doc, err := t.DataClient.List(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		assert.Equal(tt, response, res)
	}

	return Result{
		Models:   res,
		Document: doc,
		Error:    err,
	}
}

// ListError will list the provided model and expect an error.
func (t *Tester) ListError(tt *testing.T, model coal.Model, e error) Result {
	// list resources
	_, doc, err := t.DataClient.List(model)
	assert.Error(tt, err)

	// validate error
	if e != nil {
		assert.Equal(tt, e, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Find will find the provided model and validate the response if requested.
func (t *Tester) Find(tt *testing.T, model coal.Model, response coal.Model) Result {
	// find resource
	res, doc, err := t.DataClient.Find(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		assert.Equal(tt, response, res)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// FindError will find the provided model and expect and error.
func (t *Tester) FindError(tt *testing.T, model coal.Model, e error) Result {
	// find model
	_, doc, err := t.DataClient.Find(model)
	assert.Error(tt, err)

	// validate error
	if e != nil {
		assert.Equal(tt, e, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Create will create the provided model and validate the response and result
// if requested.
func (t *Tester) Create(tt *testing.T, model, response, result coal.Model) Result {
	// create resource
	res, doc, err := t.DataClient.Create(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		response.GetBase().DocID = res.ID()
		assert.Equal(tt, response, res)
	}

	// validate result
	if err == nil && result != nil {
		result.GetBase().DocID = res.ID()
		actual := coal.GetMeta(model).Make()
		t.Tester.Fetch(actual, res.ID())
		assert.Equal(tt, result, actual)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// CreateError will create the provided model and expect and error.
func (t *Tester) CreateError(tt *testing.T, model coal.Model, e error) Result {
	// create resource
	_, doc, err := t.DataClient.Create(model)
	assert.Error(tt, err)

	// validate error
	if e != nil {
		assert.Equal(tt, e, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Update will update the provided model and validate the response and result
// if requested.
func (t *Tester) Update(tt *testing.T, model, response, result coal.Model) Result {
	// update resource
	res, doc, err := t.DataClient.Update(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		assert.Equal(tt, response, res)
	}

	// validate result
	if err == nil && result != nil {
		actual := coal.GetMeta(model).Make()
		t.Tester.Fetch(actual, model.ID())
		assert.Equal(tt, result, actual)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// UpdateError will update the provided model and expect an error.
func (t *Tester) UpdateError(tt *testing.T, model coal.Model, e error) Result {
	// update resource
	_, doc, err := t.DataClient.Update(model)
	assert.Error(tt, err)

	// validate error
	if e != nil {
		assert.Equal(tt, e, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Delete will delete the provided model and validate the result.
func (t *Tester) Delete(tt *testing.T, model, result coal.Model) Result {
	// delete resource
	err := t.DataClient.Delete(model)
	assert.NoError(tt, err)

	// validate result
	if err == nil && result != nil {
		actual := coal.GetMeta(model).Make()
		t.Tester.Fetch(actual, model.ID())
		assert.Equal(tt, result, actual)
	}

	return Result{
		Error: err,
	}
}

// DeleteError will delete the provided model and expect an error.
func (t *Tester) DeleteError(tt *testing.T, model coal.Model, e error) Result {
	// delete resource
	err := t.DataClient.Delete(model)
	assert.Error(tt, err)

	// validate error
	if e != nil {
		assert.Equal(tt, e, err)
	}

	return Result{
		Error: err,
	}
}

// Upload will upload the specified data with the provided media type and name.
func (t *Tester) Upload(tt *testing.T, data []byte, typ, name string) string {
	// prepare request
	req, err := http.NewRequest("POST", t.UploadURL, bytes.NewReader(data))
	assert.NoError(tt, err)

	// set headers
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))
	if typ != "" {
		req.Header.Set("Content-Type", typ)
	}
	if name != "" {
		req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	}

	// perform request
	res, err := t.RawClient.Do(req)
	assert.NoError(tt, err)
	assert.Equal(tt, http.StatusOK, res.StatusCode)

	// get key
	var out struct {
		Keys []string `json:"keys"`
	}
	err = json.NewDecoder(res.Body).Decode(&out)
	assert.NoError(tt, err)
	assert.Len(tt, out.Keys, 1)

	return out.Keys[0]
}

// Download will download data using the specified view key. It will verify the
// files media type, name and data if requested.
func (t *Tester) Download(tt *testing.T, key string, typ, name string, data []byte) []byte {
	// prepare request
	req, err := http.NewRequest("GET", t.DownloadURL+"?dl=1&key="+key, nil)
	assert.NoError(tt, err)

	// perform request
	res, err := t.RawClient.Do(req)
	assert.NoError(tt, err)
	assert.Equal(tt, http.StatusOK, res.StatusCode)

	// check headers
	if typ != "" {
		assert.Equal(tt, typ, res.Header.Get("Content-Type"))
	}
	if name != "" {
		_, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
		assert.NoError(tt, err)
		assert.Equal(tt, name, params["filename"])
	}

	// read body
	buf, err := io.ReadAll(res.Body)
	assert.NoError(tt, err)

	// check data
	if data != nil {
		assert.Equal(tt, data, buf)
	}

	return buf
}
