package fire

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
)

// A Tester provides facilities to the test a fire API.
type Tester struct {
	// The store to use for cleaning the database.
	Store *coal.Store

	// The registered models.
	Models []coal.Model

	// The handler to be tested.
	Handler http.Handler

	// A path prefix e.g. 'api'.
	Prefix string

	// The headers to be set on all requests.
	Headers map[string]string
}

// Register will register the specified model with the tester.
func (t *Tester) Register(model coal.Model) {
	t.Models = append(t.Models, model)
}

// Clean will remove the collections of models that have been registered.
func (t *Tester) Clean() {
	store := t.Store.Copy()
	defer store.Close()

	for _, model := range t.Models {
		// remove all is faster than dropping the collection
		store.C(model).RemoveAll(nil)
	}
}

// Save will save the specified model.
func (t *Tester) Save(model coal.Model) coal.Model {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = coal.Init(model)

	// insert to collection
	err := store.C(model).Insert(model)
	if err != nil {
		panic(err)
	}

	return model
}

// SaveAll will save the specified models.
func (t *Tester) SaveAll(models ...coal.Model) {
	store := t.Store.Copy()
	defer store.Close()

	// loop through all models
	for _, model := range models {
		// initialize model
		model = coal.Init(model)

		// insert to collection
		err := store.C(model).Insert(model)
		if err != nil {
			panic(err)
		}
	}
}

// FindLast will return the last saved model.
func (t *Tester) FindLast(model coal.Model) coal.Model {
	store := t.Store.Copy()
	defer store.Close()

	err := store.C(model).Find(nil).Sort("-_id").One(model)
	if err != nil {
		panic(err)
	}

	return coal.Init(model)
}

// Path returns a root prefixed path for the supplied path.
func (t *Tester) Path(path string) string {
	// add root slash
	path = "/" + strings.Trim(path, "/")

	// add prefix if available
	if t.Prefix != "" {
		path = "/" + t.Prefix + path
	}

	return path
}

// RunValidator is a helper to test validators. The caller should assert the
// returned error of the validator, the state of the supplied model and maybe
// other objects in the database.
func (t *Tester) RunValidator(action Action, model coal.Model, validator Callback) error {
	// check action
	if action.Read() {
		panic("validator are only run on create, update and delete")
	}

	// get store
	store := t.Store.Copy()
	defer store.Close()

	// init model if present
	if model != nil {
		coal.Init(model)
	}

	// create context
	ctx := &Context{
		Action: action,
		Model:  model,
		Store:  store,
	}

	// call validator
	return validator(ctx)
}

// Request will run the specified request against the registered handler. This
// function can be used to create custom testing facilities.
func (t *Tester) Request(method, path string, payload string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	// create request
	request, err := http.NewRequest(method, t.Path(path), strings.NewReader(payload))
	if err != nil {
		panic(err)
	}

	// prepare recorder
	recorder := httptest.NewRecorder()

	// preset jsonapi accept header
	request.Header.Set("Accept", jsonapi.MediaType)

	// add content type if required
	if method == "POST" || method == "PATCH" || method == "DELETE" {
		request.Header.Set("Content-Type", jsonapi.MediaType)
	}

	// set custom headers
	for k, v := range t.Headers {
		request.Header.Set(k, v)
	}

	// server request
	t.Handler.ServeHTTP(recorder, request)

	// run callback
	callback(recorder, request)
}

// DebugRequest returns a string of information to debug requests.
func (t *Tester) DebugRequest(r *http.Request, rr *httptest.ResponseRecorder) string {
	return fmt.Sprintf(`
	URL:    %s
	Header: %s
	Status: %d
	Header: %v
	Body:   %v`, r.URL.String(), r.Header, rr.Code, rr.HeaderMap, rr.Body.String())
}
