package tools

import (
	"net/http"
	"strconv"

	"github.com/rs/cors"
)

// DefaultProtector constructs a middleware that by default limits the request
// body size to 4Ks and sets a basic CORS configuration.
func DefaultProtector() func(http.Handler) http.Handler {
	return NewProtector("4K", cors.Options{
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
			r.Body = http.MaxBytesReader(w, r.Body, parseHumanSize(maxBody))

			// call next handler
			next.ServeHTTP(w, r)
		}))
	}
}

func parseHumanSize(str string) int64 {
	const msg = "size must be like 4K, 20M or 5G"

	if len(str) < 2 {
		panic(msg)
	}

	sym := string(str[len(str)-1])

	num, err := strconv.Atoi(str[:len(str)-1])
	if err != nil {
		panic(msg)
	}

	switch sym {
	case "K":
		return int64(num) * 1000
	case "M":
		return int64(num) * 1000 * 1000
	case "G":
		return int64(num) * 1000 * 1000 * 1000
	default:
		panic(msg)
	}
}
