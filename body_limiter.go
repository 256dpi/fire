package fire

import (
	"io"
	"net/http"
)

// BodyLimiter wraps a request body and enforces read limits.
type BodyLimiter struct {
	io.ReadCloser

	// Original holds the original request body.
	Original io.ReadCloser
}

// NewBodyLimiter returns a new body limiter for the specified request.
func NewBodyLimiter(w http.ResponseWriter, r *http.Request, n int64) *BodyLimiter {
	return &BodyLimiter{
		Original:   r.Body,
		ReadCloser: http.MaxBytesReader(w, r.Body, n),
	}
}

// LimitBody will limit reading from the body of the supplied request to the
// specified amount of bytes. Earlier calls to LimitBody will be overwritten
// which essentially allows callers to increase the limit from a default limit.
func LimitBody(w http.ResponseWriter, r *http.Request, n int64) {
	// get original read from existing limiter
	if bl, ok := r.Body.(*BodyLimiter); ok {
		r.Body = bl.Original
	}

	// set new limiter
	r.Body = NewBodyLimiter(w, r, n)
}
