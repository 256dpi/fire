package fire

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompose(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("H"))
	})

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("1"))
			next.ServeHTTP(w, r)
		})
	}

	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("2"))
			next.ServeHTTP(w, r)
		})
	}

	e := Compose(h, m1, m2)

	r, err := http.NewRequest("GET", "/foo", nil)
	assert.NoError(t, err)

	w := httptest.NewRecorder()

	e.ServeHTTP(w, r)
	assert.Equal(t, "12H", w.Body.String())
}
