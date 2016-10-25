package components

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAsset(t *testing.T) {
	as := DefaultAssetServer("../.test/assets/")

	testRequest(as, "GET", "/", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(as, "GET", "/foo", func(r *httptest.ResponseRecorder, _ *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})
}

func TestAssetServer(t *testing.T) {
	as := NewAssetServer("/foo", "../.test/assets/")

	testRequest(as, "GET", "/foo/", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})

	testRequest(as, "GET", "/foo/foo", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, 200, r.Code)
		assert.Equal(t, "<h1>Hello</h1>\n", r.Body.String())
	})
}
