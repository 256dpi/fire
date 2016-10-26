package tools

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// DefaultRequestLogger constructs a handler that logs incoming requests to
// the operating systems standard error output.
func DefaultRequestLogger() func(http.Handler) http.Handler {
	return NewRequestLogger(os.Stderr)
}

// NewRequestLogger constructs a handler that logs incoming requests to the
// specified writer output.
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
			fmt.Fprintf(out, "[%s] (%d) %s - %s\n", r.Method, wrw.status, r.URL.Path, duration)
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

func (w *wrappedResponseWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func (w *wrappedResponseWriter) WriteHeader(statusCode int) {
	// Store the status code
	w.status = statusCode

	// Write the status code onward.
	w.ResponseWriter.WriteHeader(statusCode)
}
