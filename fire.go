package fire

import "net/http"

// Compose is a short-hand for chaining the specified middleware and handler
// together.
func Compose(chain ...interface{}) http.Handler {
	// check length
	if len(chain) < 2 {
		panic("fire: expected chain to have at least two items")
	}

	// get handler
	h, ok := chain[len(chain)-1].(http.Handler)
	if !ok {
		panic(`fire: expected last chain item to be a "http.Handler"`)
	}

	// chain all middleware
	for i := len(chain) - 2; i >= 0; i-- {
		// get middleware
		m, ok := chain[i].(func(http.Handler) http.Handler)
		if !ok {
			panic(`fire: expected intermediary chain item to be a "func(http.handler) http.Handler"`)
		}

		// chain
		h = m(h)
	}

	return h
}
