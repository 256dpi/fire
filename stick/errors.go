package stick

import (
	"fmt"
	"io"

	"github.com/pkg/errors"
)

// ErrorfFunc is used by F to format errors.
var ErrorfFunc = errors.Errorf

// WrapFunc is used by W to wrap errors.
var WrapFunc = errors.WithStack

// WrapfFunc is used by FW to format and wrap errors.
var WrapfFunc = errors.Wrapf

// F formats and error using ErrorfFunc.
func F(format string, args ...interface{}) error {
	return ErrorfFunc(format, args...)
}

// W wraps and error using WrapFunc.
func W(err error) error {
	return WrapFunc(err)
}

// WF formats and wraps and error using WrapfFunc.
func WF(err error, format string, args ...interface{}) error {
	return WrapfFunc(err, format, args...)
}

// SafeError wraps an error to indicate presentation safety.
type SafeError struct {
	Err error
}

// E is a short-hand function to construct a safe error.
func E(format string, args ...interface{}) error {
	return Safe(F(format, args...))
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

// Format implements the fmt.Formatter interface.
func (err *SafeError) Format(s fmt.State, verb rune) {
	// check if err implements formatter
	if fErr, ok := err.Err.(fmt.Formatter); ok {
		fErr.Format(s, verb)
		return
	}

	// write string
	_, _ = io.WriteString(s, err.Error())
}

// IsSafe can be used to check if an error has been wrapped using Safe. It will
// also detect further wrapped safe errors.
func IsSafe(err error) bool {
	return AsSafe(err) != nil
}

// AsSafe will return the safe error from an error chain.
func AsSafe(err error) *SafeError {
	var safeErr *SafeError
	errors.As(err, &safeErr)
	return safeErr
}
