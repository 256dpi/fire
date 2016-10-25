package fire

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"bytes"

	"github.com/stretchr/testify/assert"
)

func TestNewRequestLogger(t *testing.T) {
	buf := new(bytes.Buffer)

	logger := NewRequestLogger(buf)

	endpoint := func(w http.ResponseWriter, r *http.Request){
		w.WriteHeader(http.StatusContinue)
		w.Write([]byte("OK"))
	}

	handler := logger(http.HandlerFunc(endpoint))

	r, err := http.NewRequest("GET", "foo", nil)
	assert.NoError(t, err)

	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, r)

	assert.Contains(t, buf.String(), "[GET] (100) foo - ")
}

func TestDefaultRequestLogger(t *testing.T) {
	DefaultRequestLogger()
}
