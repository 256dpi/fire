// Package wood implements helpful tools that support the developer experience.
package wood

import (
	"net/http"

	"github.com/256dpi/fire"

	"github.com/goware/cors"
)

// DefaultProtector constructs a middleware that by default limits the request
// body size to 8M and sets a basic CORS configuration.
func DefaultProtector() func(http.Handler) http.Handler {
	return NewProtector("8M", cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Origin", "Accept", "Content-Type", "Authorization"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE"},
	})
}

// NewProtector constructs a middleware that implements basic protection measures
// for the passed endpoint. Currently the protector limits the body size
// to a the passed length and automatically handles CORS using the specified
// options.
func NewProtector(maxBody string, corsOptions cors.Options) func(http.Handler) http.Handler {
	c := cors.New(corsOptions)

	return func(next http.Handler) http.Handler {
		return c.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// replace read with a limited reader
			r.Body = http.MaxBytesReader(w, r.Body, int64(fire.DataSize(maxBody)))

			// call next handler
			next.ServeHTTP(w, r)
		}))
	}
}
