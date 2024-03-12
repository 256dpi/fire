package roast

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/serve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// TODO: Support filters and sorters.
// TODO: Support pagination.

// AccessDenied is the raw access denied error value.
var AccessDenied = fire.ErrAccessDenied.Self()

// ResourceNotFound is the raw resource not found error value.
var ResourceNotFound = fire.ErrResourceNotFound.Self()

// DocumentNotUnique is thr raw document not unique error value.
var DocumentNotUnique = fire.ErrDocumentNotUnique.Self()

// Config provides configuration of a tester.
type Config struct {
	Tester           *fire.Tester
	Store            *coal.Store  // ignored, if Tester is provided
	Models           []coal.Model // ignored, if Tester is provided
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
	Config     Config
	RawClient  *http.Client
	DataClient *fire.Client
	AuthClient *oauth2.Client
	AuthToken  string
}

// NewTester will create and return a new tester.
func NewTester(config Config) *Tester {
	// prepare tester
	tester := &Tester{
		Tester: config.Tester,
	}

	// ensure parent tester
	if tester.Tester == nil {
		tester.Tester = fire.NewTester(config.Store, config.Models...)
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

	// set config
	tester.Config = config

	return tester
}

// URL returns a URL based on the specified path segments.
func (t *Tester) URL(path ...string) string {
	return strings.ReplaceAll("/"+strings.Trim(t.Config.DataNamespace, "/")+"/"+strings.Join(path, "/"), "//", "/")
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
func (t *Tester) List(tt *testing.T, model coal.Model, response []coal.Model, hooks ...func(res, cmp coal.Model)) Result {
	// list resources
	res, doc, err := t.DataClient.List(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		for i := range response {
			for _, cb := range hooks {
				cb(res[i], response[i])
			}
		}
		assert.Equal(tt, response, res)
	}

	return Result{
		Models:   res,
		Document: doc,
		Error:    err,
	}
}

// ListError will list the provided model and expect an error.
func (t *Tester) ListError(tt *testing.T, model coal.Model, expected error) Result {
	// list resources
	_, doc, err := t.DataClient.List(model)
	assert.Error(tt, err)

	// validate error
	if expected != nil {
		assert.Equal(tt, expected, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Find will find the provided model and validate the response if requested.
func (t *Tester) Find(tt *testing.T, model coal.Model, response coal.Model, hooks ...func(res, cmp coal.Model)) Result {
	// find resource
	res, doc, err := t.DataClient.Find(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		for _, cb := range hooks {
			cb(res, response)
		}
		assert.Equal(tt, response, res)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// FindError will find the provided model and expect and error.
func (t *Tester) FindError(tt *testing.T, model coal.Model, expected error) Result {
	// find model
	_, doc, err := t.DataClient.Find(model)
	assert.Error(tt, err)

	// validate error
	if expected != nil {
		assert.Equal(tt, expected, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Create will create the provided model and validate the response and result
// if requested.
func (t *Tester) Create(tt *testing.T, model, response, result coal.Model, hooks ...func(res, cmp coal.Model)) Result {
	// create resource
	res, doc, err := t.DataClient.Create(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		response.GetBase().DocID = res.ID()
		for _, cb := range hooks {
			cb(res, response)
		}
		assert.Equal(tt, response, res)
	}

	// validate result
	if err == nil && result != nil {
		result.GetBase().DocID = res.ID()
		actual := coal.GetMeta(model).Make()
		t.Tester.Fetch(actual, res.ID())
		for _, cb := range hooks {
			cb(actual, result)
		}
		assert.Equal(tt, result, actual)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// CreateError will create the provided model and expect and error.
func (t *Tester) CreateError(tt *testing.T, model coal.Model, expected error) Result {
	// create resource
	_, doc, err := t.DataClient.Create(model)
	assert.Error(tt, err)

	// validate error
	if expected != nil {
		assert.Equal(tt, expected, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Update will update the provided model and validate the response and result
// if requested.
func (t *Tester) Update(tt *testing.T, model, response, result coal.Model, hooks ...func(res, cmp coal.Model)) Result {
	// update resource
	res, doc, err := t.DataClient.Update(model)
	assert.NoError(tt, err)

	// validate response
	if err == nil && response != nil {
		for _, cb := range hooks {
			cb(res, response)
		}
		assert.Equal(tt, response, res)
	}

	// validate result
	if err == nil && result != nil {
		actual := coal.GetMeta(model).Make()
		t.Tester.Fetch(actual, model.ID())
		for _, cb := range hooks {
			cb(actual, result)
		}
		assert.Equal(tt, result, actual)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// UpdateError will update the provided model and expect an error.
func (t *Tester) UpdateError(tt *testing.T, model coal.Model, expected error) Result {
	// update resource
	_, doc, err := t.DataClient.Update(model)
	assert.Error(tt, err)

	// validate error
	if expected != nil {
		assert.Equal(tt, expected, err)
	}

	return Result{
		Document: doc,
		Error:    err,
	}
}

// Delete will delete the provided model and validate the result.
func (t *Tester) Delete(tt *testing.T, model, result coal.Model, hooks ...func(res, cmp coal.Model)) Result {
	// delete resource
	err := t.DataClient.Delete(model)
	assert.NoError(tt, err)

	// validate result
	if err == nil && result != nil {
		actual := coal.GetMeta(model).Make()
		t.Tester.Fetch(actual, model.ID())
		for _, fn := range hooks {
			fn(actual, result)
		}
		assert.Equal(tt, result, actual)
	}

	return Result{
		Error: err,
	}
}

// DeleteError will delete the provided model and expect an error.
func (t *Tester) DeleteError(tt *testing.T, model coal.Model, expected error) Result {
	// delete resource
	err := t.DataClient.Delete(model)
	assert.Error(tt, err)

	// validate error
	if expected != nil {
		assert.Equal(tt, expected, err)
	}

	return Result{
		Error: err,
	}
}

// Call will call a JSON based action with the provided input and output at
// the specified URL. If out is absent, the function will try to decode and
// return any jsonapi.Error.
func (t *Tester) Call(tt *testing.T, url string, in, out any) (int, *jsonapi.Error) {
	// encode request
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		require.NoError(tt, err)
		body = bytes.NewReader(data)
	}

	// prepare request
	req, err := http.NewRequest("POST", url, body)
	require.NoError(tt, err)

	// set header
	req.Header.Set("Content-Type", "application/json")

	// run authorizer if available
	if t.Config.Authorizer != nil {
		t.Config.Authorizer(req)
	}

	// perform request
	res, err := t.RawClient.Do(req)
	require.NoError(tt, err)
	defer res.Body.Close()

	// decode response
	if out != nil {
		err = json.NewDecoder(res.Body).Decode(out)
		require.NoError(tt, err)
	} else {
		var doc jsonapi.Document
		_ = json.NewDecoder(res.Body).Decode(&doc)
		if len(doc.Errors) > 0 {
			return res.StatusCode, doc.Errors[0]
		}
	}

	return res.StatusCode, nil
}

// Upload will upload the specified data with the provided media type and name.
func (t *Tester) Upload(tt *testing.T, data []byte, typ, name string) string {
	// prepare request
	req, err := http.NewRequest("POST", t.URL(t.Config.UploadEndpoint), bytes.NewReader(data))
	assert.NoError(tt, err)

	// set headers
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))
	if typ != "" {
		req.Header.Set("Content-Type", typ)
	}
	if name != "" {
		req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename*=utf-8''%s", url.QueryEscape(name)))
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
	req, err := http.NewRequest("GET", t.URL(t.Config.DownloadEndpoint)+"?dl=1&key="+key, nil)
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

// Await will wait for all jobs created during the execution of the callback to
// complete. A timeout may be provided to stop after some time.
func (t *Tester) Await(tt *testing.T, timeout time.Duration, fns ...func()) int {
	// wrap functions
	var newFNs []func() error
	for _, fn := range fns {
		newFNs = append(newFNs, func() error {
			fn()
			return nil
		})
	}

	// await processing
	n, err := axe.Await(t.Store, timeout, newFNs...)
	assert.NoError(tt, err)

	return n
}

// CopyHook will copy fields from the response to the result.
func CopyHook(fields ...string) func(res, cmp coal.Model) {
	return func(res, cmp coal.Model) {
		for _, field := range fields {
			stick.MustSet(cmp, field, stick.MustGet(res, field))
		}
	}
}

// SkipHook will copy fields from the result to the response.
func SkipHook(fields ...string) func(res, cmp coal.Model) {
	return func(res, cmp coal.Model) {
		for _, field := range fields {
			stick.MustSet(res, field, stick.MustGet(cmp, field))
		}
	}
}
