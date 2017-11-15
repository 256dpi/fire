package wood

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
)

func testRequest(h http.Handler, method, path string, headers map[string]string, payload string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	r, err := http.NewRequest(method, path, strings.NewReader(payload))
	if err != nil {
		panic(err)
	}

	w := httptest.NewRecorder()

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	h.ServeHTTP(w, r)

	callback(w, r)
}

func randomReader(size int) io.Reader {
	b := make([]byte, size)
	rand.Read(b)
	return bytes.NewReader(b)
}
