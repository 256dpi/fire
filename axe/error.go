package axe

// Error is used to signal failed job executions.
type Error struct {
	Reason string
	Retry  bool
}

// E is a short-hand to construct an error.
func E(reason string, retry bool) *Error {
	return &Error{
		Reason: reason,
		Retry:  retry,
	}
}

// Error implements the error interface.
func (c *Error) Error() string {
	return c.Reason
}
