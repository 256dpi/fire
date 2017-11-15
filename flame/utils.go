package flame

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func debugRequest(r *http.Request, rr *httptest.ResponseRecorder) string {
	return fmt.Sprintf(`
	URL:    %s
	Header: %s
	Status: %d
	Header: %v
	Body:   %v`, r.URL.String(), r.Header, rr.Code, rr.HeaderMap, rr.Body.String())
}
