package components

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gonfire/fire"
	"github.com/pressly/chi"
	"github.com/stretchr/testify/assert"
)

func TestDefaultAsset(t *testing.T) {
	as := DefaultAssetServer("../.test/assets/")

	assert.Equal(t, fire.ComponentInfo{
		Name: "Asset Server",
		Settings: fire.Map{
			"SPA Mode":  "true",
			"Path":      "",
			"Directory": "../.test/assets/",
		},
	}, as.Describe())

	router := chi.NewRouter()

	as.Register(nil, router)

	testRequest(router, "GET", "/", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(router, "GET", "/foo", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})
}

func TestAssetServer(t *testing.T) {
	as := NewAssetServer("/foo", "../.test/assets/", true)

	assert.Equal(t, fire.ComponentInfo{
		Name: "Asset Server",
		Settings: fire.Map{
			"SPA Mode":  "true",
			"Path":      "/foo",
			"Directory": "../.test/assets/",
		},
	}, as.Describe())

	router := chi.NewRouter()

	as.Register(nil, router)

	testRequest(router, "GET", "/foo/", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(router, "GET", "/foo/foo", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})
}
