package components

import (
	"net/http"
	"net/http/httptest"
)

func testRequest(h http.Handler, method, path string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	r, err := http.NewRequest(method, path, nil)
	if err != nil {
		panic(err)
	}

	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, r)

	callback(rec, r)
}
