package fire

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// A Tester provides facilities to the test a fire API.
type Tester struct {
	*coal.Tester

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
		Tester:  coal.NewTester(store, models...),
		Header:  make(map[string]string),
		Context: context.Background(),
	}
}

// Assign will create a controller group with the specified controllers and
// assign in to the Handler attribute of the tester. It will return the created
// group.
func (t *Tester) Assign(prefix string, controllers ...*Controller) *Group {
	// create group
	group := NewGroup(func(err error) {
		panic(err)
	})

	// ensure store
	for _, c := range controllers {
		if c.Store == nil {
			c.Store = t.Store
		}
	}

	// add controllers
	group.Add(controllers...)

	// set handler
	t.Handler = serve.Compose(xo.RootHandler(), group.Endpoint(prefix))

	return group
}

// Clean will remove the collections of models that have been registered and
// reset the header map as well as the context.
func (t *Tester) Clean() {
	// clean models
	t.Tester.Clean()

	// reset header
	t.Header = make(map[string]string)

	// reset context
	t.Context = context.Background()
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
	return t.RunHandler(ctx, func(ctx *Context) error {
		// force stage
		ctx.Stage = Authorizer
		if cb.Stage != 0 {
			ctx.Stage = cb.Stage.Split()[0]
		}

		// check matcher
		if !cb.Matcher(ctx) {
			return fmt.Errorf("failed matcher")
		}

		return cb.Handler(ctx)
	})
}

// RunAction is a helper to test actions.
func (t *Tester) RunAction(ctx *Context, action *Action) (*httptest.ResponseRecorder, error) {
	// set default
	if action.BodyLimit <= 0 {
		action.BodyLimit = serve.MustByteSize("8M")
	}

	// get context
	var rec *httptest.ResponseRecorder
	err := t.RunHandler(ctx, func(ctx *Context) error {
		// get response recorder
		rec = ctx.ResponseWriter.(*httptest.ResponseRecorder)

		// check method
		if !stick.Contains(action.Methods, ctx.HTTPRequest.Method) {
			rec.WriteHeader(http.StatusMethodNotAllowed)
			return nil
		}

		// limit request
		serve.LimitBody(rec, ctx.HTTPRequest, action.BodyLimit)

		// run action
		return action.Handler(ctx)
	})
	if err != nil {
		rec.WriteHeader(http.StatusInternalServerError)
	}

	return rec, err
}

// RunHandler builds a context and runs the passed handler with it.
func (t *Tester) RunHandler(ctx *Context, h Handler) error {
	return t.WithContext(ctx, func(ctx *Context) error {
		return h(ctx)
	})
}

// WithContext runs the given function with a prepared context.
func (t *Tester) WithContext(ctx *Context, fn func(ctx *Context) error) error {
	// ensure context
	if ctx == nil {
		ctx = &Context{}
	}

	// ensure context
	if ctx.Context == nil {
		ctx.Context = t.Context
	}

	// ensure data
	if ctx.Data == nil {
		ctx.Data = stick.Map{}
	}

	// ensure stage
	if ctx.Stage == 0 {
		ctx.Stage = Authorizer
	}

	// ensure operation
	if ctx.Operation == 0 {
		ctx.Operation = List
	}

	// ensure selector
	if ctx.Selector == nil {
		ctx.Selector = bson.M{}
	}

	// ensure filters
	if ctx.Filters == nil {
		ctx.Filters = []bson.M{}
	}

	// ensure relationship filters
	if ctx.RelationshipFilters == nil {
		ctx.RelationshipFilters = map[string][]bson.M{}
	}

	// set store if unset
	if ctx.Store == nil {
		ctx.Store = t.Store
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

	// create trace
	ctx.Tracer, ctx.Context = xo.CreateTracer(ctx.Context, "fire/Tester.WithContext")
	defer ctx.Tracer.End()

	// yield context
	if !ctx.Operation.Action() && ctx.Store != nil {
		return ctx.Store.T(ctx.Context, false, func(tc context.Context) error {
			return ctx.With(tc, func() error {
				return fn(ctx)
			})
		})
	}

	return fn(ctx)
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
	Body:   %v`, r.URL.String(), r.Header, rr.Code, rr.Result().Header, rr.Body.String())
}
