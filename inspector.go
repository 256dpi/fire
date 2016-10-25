package fire

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/pressly/chi"
)

func PrintRoutes(router chi.Router) {
	// prepare routes
	var routes []string

	// add all routes as string
	for _, route := range router.Routes() {
		for method := range route.Handlers {
			routes = append(routes, fmt.Sprintf("%6s  %-30s", method, route.Pattern))
		}
	}

	// sort routes
	sort.Strings(routes)

	// print routes
	for _, route := range routes {
		fmt.Println(route)
	}
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// wrap response writer
		wrw := wrapResponseWriter(w)

		// save start
		start := time.Now()

		// call next handler
		next.ServeHTTP(wrw, r)

		// get request duration
		duration := time.Since(start).String()

		// log request
		fmt.Printf("%6s  %s\n   %d  %s\n", r.Method, r.URL.Path, wrw.Status(), duration)
	})
}

type wrappedResponseWriter struct {
	status int
	http.ResponseWriter
}

func wrapResponseWriter(res http.ResponseWriter) *wrappedResponseWriter {
	// default the status code to 200
	return &wrappedResponseWriter{200, res}
}

func (w wrappedResponseWriter) Status() int {
	return w.status
}

func (w wrappedResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w wrappedResponseWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func (w wrappedResponseWriter) WriteHeader(statusCode int) {
	// Store the status code
	w.status = statusCode

	// Write the status code onward.
	w.ResponseWriter.WriteHeader(statusCode)
}
