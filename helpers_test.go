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

	e := Compose(m1, m2, h)

	r, err := http.NewRequest("GET", "/foo", nil)
	assert.NoError(t, err)

	w := httptest.NewRecorder()

	e.ServeHTTP(w, r)
	assert.Equal(t, "12H", w.Body.String())
}

func TestComposePanics(t *testing.T) {
	assert.Panics(t, func() {
		Compose()
	})

	assert.Panics(t, func() {
		Compose(nil, nil)
	})

	assert.Panics(t, func() {
		Compose(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	})
}

func TestDataSize(t *testing.T) {
	assert.Equal(t, uint64(50*1000), DataSize("50K"))
	assert.Equal(t, uint64(5*1000*1000), DataSize("5M"))
	assert.Equal(t, uint64(100*1000*1000*1000), DataSize("100G"))

	for _, str := range []string{"", "1", "K", "10", "KM"} {
		assert.Panics(t, func() {
			DataSize(str)
		})
	}
}
