package wood

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProtectorBodyOverflow(t *testing.T) {
	p := DefaultProtector()
	e := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := ioutil.ReadAll(r.Body)
		assert.Equal(t, 8000000, len(buf))
		assert.Error(t, err)

		w.WriteHeader(http.StatusContinue)
		_, _ = w.Write([]byte("OK"))
	})
	h := p(e)

	r, err := http.NewRequest("GET", "/foo", randomReader(8000001))
	assert.NoError(t, err)

	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusContinue, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestProtectorCORS(t *testing.T) {
	p := DefaultProtector()
	e := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusContinue)
		_, _ = w.Write([]byte("OK"))
	})
	h := p(e)

	r, err := http.NewRequest("OPTIONS", "/foo", nil)
	assert.NoError(t, err)

	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, http.Header{
		"Vary": []string{
			"Origin",
			"Access-Control-Request-Method",
			"Access-Control-Request-Headers",
		},
	}, w.HeaderMap)
	assert.Empty(t, w.Body.String())
}
