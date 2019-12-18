package fire

import (
	"fmt"
)

// Map is a general purpose type to represent a map.
type Map map[string]interface{}

type safeError struct {
	err error
}

// E is a short-hand function to construct a safe error.
func E(format string, a ...interface{}) error {
	return Safe(fmt.Errorf(format, a...))
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

// Unique is a helper to get a unique list of object ids.
func Unique(list []string) []string {
	// prepare map and list
	m := make(map[string]bool)
	l := make([]string, 0, len(list))

	// add items not present in map
	for _, id := range list {
		if _, ok := m[id]; !ok {
			m[id] = true
			l = append(l, id)
		}
	}

	return l
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

// Includes returns true if a list of strings includes another list of strings.
func Includes(all, subset []string) bool {
	for _, item := range subset {
		if !Contains(all, item) {
			return false
		}
	}

	return true
}

// Union will return a unique list with items from both lists.
func Union(listA, listB []string) []string {
	// prepare new list
	list := make([]string, 0, len(listA)+len(listB))
	list = append(list, listA...)
	list = append(list, listB...)

	// return unique list
	return Unique(list)
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
