package fire

// Cause is a wrapper around errors handled by Assert, Panic and Recover.
type Cause struct {
	Err error
}

// Assert will only abort if the supplied error is present and does not belong
// to the specified whitelist.
func Assert(err error) {
	if err != nil {
		Abort(err)
	}
}

// Abort will abort even if the supplied error is nil.
func Abort(err error) {
	panic(&Cause{err})
}

// Continue will try to recover an earlier call to Abort and call fn if an error
// has been recovered. It will not recover direct calls to the built-in panic.
//
// Note: If the built-in panic has been called with nil a call to Continue will
// discard that panic and continue execution.
func Continue(fn func(error)) {
	val := recover()
	if cause, ok := val.(*Cause); ok {
		fn(cause.Err)
		return
	} else if val != nil {
		panic(val)
	}
}
