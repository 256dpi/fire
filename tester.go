package fire

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
	"github.com/globalsign/mgo/bson"
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

	// The header to be set on all requests and contexts.
	Header map[string]string

	// Context to be set on fake requests.
	Context context.Context
}

// NewTester returns a new tester.
func NewTester(store *coal.Store, models ...coal.Model) *Tester {
	return &Tester{
		Store:   store,
		Models:  models,
		Header:  make(map[string]string),
		Context: context.Background(),
	}
}

// Assign will create a controller group with the specified controllers and
// assign in to the Handler attribute of the tester.
func (t *Tester) Assign(prefix string, controllers ...*Controller) {
	group := NewGroup()
	group.Add(controllers...)
	group.Reporter = func(err error) {
		panic(err)
	}

	t.Handler = Compose(RootTracer(), group.Endpoint(prefix))
}

// Clean will remove the collections of models that have been registered and
// reset the header map.
func (t *Tester) Clean() {
	store := t.Store.Copy()
	defer store.Close()

	for _, model := range t.Models {
		// remove all is faster than dropping the collection
		_, err := store.C(model).RemoveAll(nil)
		if err != nil {
			panic(err)
		}
	}

	// reset header
	t.Header = make(map[string]string)

	// reset context
	t.Context = context.Background()
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

// Update will update the specified model.
func (t *Tester) Update(model coal.Model) coal.Model {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = coal.Init(model)

	// insert to collection
	err := store.C(model).UpdateId(model.ID(), model)
	if err != nil {
		panic(err)
	}

	return model
}

// Delete will delete the specified model.
func (t *Tester) Delete(model coal.Model) {
	store := t.Store.Copy()
	defer store.Close()

	// initialize model
	model = coal.Init(model)

	// insert to collection
	err := store.C(model).RemoveId(model.ID())
	if err != nil {
		panic(err)
	}
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

// RunCallback is a helper to test callbacks.
func (t *Tester) RunCallback(ctx *Context, cb *Callback) error {
	return t.RunHandler(ctx, cb.Handler)
}

// WithContext runs the given function with a prepared context.
func (t *Tester) WithContext(ctx *Context, fn func(*Context)) {
	t.RunHandler(ctx, func(ctx *Context) error {
		fn(ctx)
		return nil
	})
}

// RunHandler builds a context and runs the passed handler with it.
func (t *Tester) RunHandler(ctx *Context, h Handler) error {
	// set context if missing
	if ctx == nil {
		ctx = &Context{}
	}

	// set operation if missing
	if ctx.Operation == 0 {
		ctx.Operation = List
	}

	// set store if unset
	if ctx.Store == nil {
		ctx.Store = t.Store.Copy()
		defer ctx.Store.Close()
	}

	// init model if present
	if ctx.Model != nil {
		coal.Init(ctx.Model)
	}

	// init queries
	if ctx.Selector == nil {
		ctx.Selector = bson.M{}
	}
	if ctx.Filters == nil {
		ctx.Filters = []bson.M{}
	}

	// set request
	if ctx.HTTPRequest == nil {
		// create request
		req, err := http.NewRequest("GET", "", nil)
		if err != nil {
			panic(err)
		}

		// set headers
		for key, value := range t.Header {
			req.Header.Set(key, value)
		}

		// set context
		ctx.HTTPRequest = req.WithContext(t.Context)
	}

	// set response writer
	if ctx.ResponseWriter == nil {
		ctx.ResponseWriter = httptest.NewRecorder()
	}

	// set json api request
	if ctx.JSONAPIRequest == nil {
		ctx.JSONAPIRequest = &jsonapi.Request{}
	}

	// set tracers
	ctx.Tracer = NewTracerWithRoot("fire/Tester.RunHandler")
	defer ctx.Tracer.Finish(true)

	// call handler
	return h(ctx)
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
	for k, v := range t.Header {
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
