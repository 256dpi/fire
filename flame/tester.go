// Package flame provides tools to test fire based JSON-APIs.
package flame

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
)

// A Tester provides facilities to the test a fire based API.
type Tester struct {
	// The store to use for cleaning the database.
	Store *coal.Store

	// The registered models.
	Models []coal.Model

	// The handler to be tested.
	Handler http.Handler

	// A path prefix e.g. 'api'.
	Prefix string
}

// Register will register the specified model with the tester.
func (t *Tester) Register(model coal.Model) {
	t.Models = append(t.Models, model)
}

// CleanStore will remove the collections of models that have been registered.
func (t *Tester) CleanStore() {
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

// Request will run the specified low-level request against the current handler.
func (t *Tester) Request(method, path string, headers map[string]string, payload string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	// add root slash
	path = "/" + path

	// add prefix if available
	if t.Prefix != "" {
		path = "/" + t.Prefix + path
	}

	// create request
	request, err := http.NewRequest(method, path, strings.NewReader(payload))
	if err != nil {
		panic(err)
	}

	// prepare recorder
	recorder := httptest.NewRecorder()

	// preset jsonapi accept header
	request.Header.Set("Accept", jsonapi.MediaType)

	// set custom headers
	for k, v := range headers {
		request.Header.Set(k, v)
	}

	// server request
	t.Handler.ServeHTTP(recorder, request)

	// run callback
	callback(recorder, request)
}
