package fire

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func DefaultRequestLogger() func(http.Handler) http.Handler {
	return NewRequestLogger(os.Stderr)
}

func NewRequestLogger(out io.Writer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
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
			fmt.Fprintf(out, "[%s] (%d) %s - %s\n", r.Method, wrw.Status(), r.URL.Path, duration)
		})
	}
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
