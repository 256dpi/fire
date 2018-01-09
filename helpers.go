package fire

import (
	"net/http"
	"strconv"
)

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

// DataSize parses human readable data sizes (e.g. 4K, 20M or 5G) and returns
// the amount of bytes they represent.
func DataSize(str string) uint64 {
	const msg = "fire: data size must be like 4K, 20M or 5G"

	// check length
	if len(str) < 2 {
		panic(msg)
	}

	// get symbol
	sym := string(str[len(str)-1])

	// parse number
	num, err := strconv.ParseUint(str[:len(str)-1], 10, 64)
	if err != nil {
		panic(msg)
	}

	// calculate size
	switch sym {
	case "K":
		return num * 1000
	case "M":
		return num * 1000 * 1000
	case "G":
		return num * 1000 * 1000 * 1000
	}

	panic(msg)
}
