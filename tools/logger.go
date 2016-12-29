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

// ResponseWriter is the ResponseWriter that wraps the original ResponseWriter
// if the Logger middleware has been used in the chain.
type ResponseWriter struct {
	http.ResponseWriter

	status int
}

func wrapResponseWriter(res http.ResponseWriter) *ResponseWriter {
	// default the status code to 200
	return &ResponseWriter{res, 200}
}

// WriteHeader calls the underlying ResponseWriters WriteHeader method.
func (w *ResponseWriter) WriteHeader(statusCode int) {
	// Store the status code
	w.status = statusCode

	// Write the status code onward.
	w.ResponseWriter.WriteHeader(statusCode)
}

// UnwrapResponseWriter will try to unwrap the passed ResponseWriter.
func UnwrapResponseWriter(w http.ResponseWriter) http.ResponseWriter {
	if rw, ok := w.(*ResponseWriter); ok {
		return rw.ResponseWriter
	}

	return w
}
