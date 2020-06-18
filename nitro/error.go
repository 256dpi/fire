package nitro

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// AsError will try to unwrap an Error from err.
func AsError(err error) *Error {
	var anError *Error
	if errors.As(err, &anError) {
		return anError
	}
	return nil
}

// Error objects provide additional information about problems encountered
// while performing an RPC operation.
type Error struct {
	// A unique identifier for this particular occurrence of the problem.
	ID string `json:"id,omitempty" bson:"id,omitempty"`

	// An URL that leads to further details about this problem.
	Link string `json:"link,omitempty" bson:"link,omitempty"`

	// The HTTP status code applicable to this problem.
	Status int `json:"status,string,omitempty" bson:"status,string,omitempty"`

	// An application-specific error code.
	Code string `json:"code,omitempty" bson:"code,omitempty"`

	// A short, human-readable summary of the problem.
	Title string `json:"title,omitempty" bson:"title,omitempty"`

	// A human-readable explanation specific to this occurrence of the problem.
	Detail string `json:"detail,omitempty" bson:"detail,omitempty"`

	// A JSON pointer specifying the source of the error.
	//
	// See https://tools.ietf.org/html/rfc6901.
	Source string `json:"source,omitempty" bson:"source,omitempty"`

	// Non-standard meta-information about the error.
	Meta map[string]interface{} `json:"meta,omitempty" bson:"meta,omitempty"`
}

// Error returns a string representation of the error for logging purposes.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Title, e.Detail)
}

// ErrorFromStatus will return an error that has been derived from the passed
// status code.
//
// Note: If the passed status code is not a valid HTTP status code, an Internal
// Server Error status code will be used instead.
func ErrorFromStatus(status int, detail string) *Error {
	// get text
	str := strings.ToLower(http.StatusText(status))

	// check text
	if str == "" {
		status = http.StatusInternalServerError
		str = strings.ToLower(http.StatusText(http.StatusInternalServerError))
	}

	return &Error{
		Status: status,
		Title:  str,
		Detail: detail,
	}
}

// BadRequest returns a new bad request error with a source.
func BadRequest(detail, source string) *Error {
	err := ErrorFromStatus(http.StatusBadRequest, detail)
	err.Source = source

	return err
}
