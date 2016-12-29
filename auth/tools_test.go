package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenMigrator(t *testing.T) {
	migrator := TokenMigrator(true)

	handler := migrator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer foo", r.Header.Get("Authorization"))
		assert.Equal(t, "", r.URL.Query().Get("access_token"))

		w.Write([]byte("OK"))
	}))

	testRequest(handler, "GET", "/foo?access_token=foo", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, "OK", r.Body.String())
	})
}
