package fire

import "net/http"

// Handler is function that takes a context, mutates is to modify the behaviour
// and response or return an error.
//
// If a returned error is wrapped using Fatal, processing stops immediately and
// the error is logged.
type Handler func(*Context) error

type fatalError struct {
	err error
}

// TODO: Also add helpers for returning safe jsonapi errors.

// Fatal wraps an error and marks it as fatal.
func Fatal(err error) error {
	return &fatalError{
		err: err,
	}
}

func (err *fatalError) Error() string {
	return err.err.Error()
}

// IsFatal can be used to check if an error has been wrapped using Fatal.
func IsFatal(err error) bool {
	_, ok := err.(*fatalError)
	return ok
}

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
