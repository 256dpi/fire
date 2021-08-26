package roast

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
)

// TODO: Support Upload & Download.

// AccessDenied is the raw access denied error value.
var AccessDenied = fire.ErrAccessDenied.Self().(*xo.Err).Err

// ResourceNotFound is the raw resource not found error value.
var ResourceNotFound = fire.ErrResourceNotFound.Self().(*xo.Err).Err

// Config provides configuration of a tester.
type Config struct {
	Store         *coal.Store
	Catalog       *coal.Catalog
	Handler       http.Handler
	DataNamespace string
	AuthNamespace string
	TokenEndpoint string
	Authorizer    func(req *http.Request)
	Debug         bool
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
	RawClient  *http.Client
	DataClient *Client
	AuthClient *oauth2.Client
	AuthToken  string
}

// NewTester will create and return a new tester.
func NewTester(config Config) *Tester {
	// prepare tester
	c := &Tester{
		Tester: fire.NewTester(config.Store, config.Catalog.Models()...),
	}

	// set handler
	c.Tester.Handler = config.Handler

	// prepare http client
	c.RawClient = &http.Client{
		Transport: serve.Local(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if config.Debug {
				fmt.Println(r.Method, r.URL.String())
			}
			c.Tester.Handler.ServeHTTP(w, r)
			if config.Debug {
				rec := w.(*httptest.ResponseRecorder)
				fmt.Println(rec.Code, rec.Body.String())
			}
		})),
	}

	// get authorizer
	if config.Authorizer == nil {
		config.Authorizer = func(req *http.Request) {
			if c.AuthToken != "" {
				req.Header.Set("Authorization", "Bearer "+c.AuthToken)
			}
		}
	}

	// set data client
	c.DataClient = NewClient(jsonapi.NewClientWithClient(jsonapi.ClientConfig{
		BaseURI:       "/" + strings.Trim(config.DataNamespace, "/"),
		Authorizer:    config.Authorizer,
		ResponseLimit: serve.MustByteSize("8M"),
	}, c.RawClient))

	// set auth client
	c.AuthClient = oauth2.NewClientWithClient(oauth2.ClientConfig{
		BaseURI:       "/" + strings.Trim(config.AuthNamespace, "/"),
		TokenEndpoint: "/" + strings.Trim(config.TokenEndpoint, "/"),
	}, c.RawClient)

	return c
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
		rec := coal.GetMeta(model).Make()
		t.Tester.Fetch(rec, res.ID())
		assert.Equal(tt, result, res)
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
		base := actual.GetBase()
		*base = coal.B(actual.ID())
		assert.Equal(tt, result, actual)
	}

	return Result{
		Model:    res,
		Document: doc,
		Error:    err,
	}
}

// UpdateError will updatet the provided model and expect an error.
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
		base := actual.GetBase()
		*base = coal.B(actual.ID())
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
