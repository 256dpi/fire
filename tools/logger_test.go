package tools

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRequestLogger(t *testing.T) {
	buf := new(bytes.Buffer)

	logger := NewRequestLogger(buf)

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusContinue)
		w.Write([]byte("OK"))

		assert.True(t, UnwrapResponseWriter(w) != w)
	}

	handler := logger(http.HandlerFunc(endpoint))

	testRequest(handler, "GET", "/foo", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusContinue, r.Code)
		assert.Contains(t, buf.String(), "[GET] (100) /foo - ")
	})
}

func TestDefaultRequestLogger(t *testing.T) {
	DefaultRequestLogger()
}
