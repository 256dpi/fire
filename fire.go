package fire

import "net/http"

// Compose is a short-hand for chaining the specified middlewares and handler
// together.
func Compose(h http.Handler, m ...func(http.Handler) http.Handler) http.Handler {
	// chain all middlewares
	for i := range m {
		h = m[len(m)-1-i](h)
	}

	return h
}
