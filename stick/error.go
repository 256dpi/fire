package stick

import (
	"fmt"
)

// SafeError wraps an error to indicate presentation safety.
type SafeError struct {
	Err error
}

// E is a short-hand function to construct a safe error.
func E(format string, a ...interface{}) error {
	return Safe(fmt.Errorf(format, a...))
}

// Safe wraps an error and marks it as safe. Wrapped errors are safe to be
// presented to the client if appropriate.
func Safe(err error) error {
	return &SafeError{
		Err: err,
	}
}

// Error implements the error interface.
func (err *SafeError) Error() string {
	return err.Err.Error()
}

// Unwrap will return the wrapped error.
func (err *SafeError) Unwrap() error {
	return err.Err
}

// IsSafe can be used to check if an error has been wrapped using Safe.
func IsSafe(err error) bool {
	_, ok := err.(*SafeError)
	return ok
}
