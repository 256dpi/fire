package fire

import (
	"net/http"
	"strconv"
)

type safeError struct {
	err error
}

// Safe wraps an error and marks it as safe. Wrapped errors are safe to be
// presented to the client if appropriate.
func Safe(err error) error {
	return &safeError{
		err: err,
	}
}

func (err *safeError) Error() string {
	return err.err.Error()
}

// IsSafe can be used to check if an error has been wrapped using Safe.
func IsSafe(err error) bool {
	_, ok := err.(*safeError)
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

// Contains returns true if a list of strings contains another string.
func Contains(list []string, str string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}

	return false
}

// Intersect will return the intersection of both lists.
func Intersect(listA, listB []string) []string {
	// prepare new list
	list := make([]string, 0, len(listA))

	// add items that are part of both lists
	for _, item := range listA {
		if Contains(listB, item) {
			list = append(list, item)
		}
	}

	return list
}
