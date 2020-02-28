package ash

import "github.com/256dpi/fire"

// A is a short-hand function to construct an authorizer. It will also add tracing
// code around the execution of the authorizer.
func A(name string, m fire.Matcher, h Handler) *Authorizer {
	// panic if matcher or handler is not set
	if m == nil || h == nil {
		panic("ash: missing matcher or handler")
	}

	// construct and return authorizer
	return &Authorizer{
		Matcher: m,
		Handler: func(ctx *fire.Context) ([]*Enforcer, error) {
			// trace
			ctx.Trace.Push(name)
			defer ctx.Trace.Pop()

			// call handler
			enforcers, err := h(ctx)
			if err != nil {
				return nil, err
			}

			return enforcers, nil
		},
	}
}

// S is a short-hand for a set of enforcers.
type S = []*Enforcer

// Handler is a function that inspects an operation context and potentially
// returns a set of enforcers or an error.
type Handler func(*fire.Context) ([]*Enforcer, error)

// An Authorizer should inspect the specified context and assesses if it is able
// to enforce authorization with the data that is available. If yes, the
// authorizer should return a non zero set of enforcers that will enforce the
// authorization.
type Authorizer struct {
	// The matcher that decides whether the authorizer can be run.
	Matcher fire.Matcher

	// The handler handler that gets executed with the context.
	Handler Handler
}

// And will match and run both authorizers and return immediately if one does not
// return a set of enforcers. The two successfully returned enforcer sets are
// merged into one and returned.
func And(a, b *Authorizer) *Authorizer {
	return A("ash/And", func(ctx *fire.Context) bool {
		return a.Matcher(ctx) && b.Matcher(ctx)
	}, func(ctx *fire.Context) ([]*Enforcer, error) {
		// run first callback
		enforcers1, err := a.Handler(ctx)
		if err != nil {
			return nil, err
		} else if len(enforcers1) == 0 {
			return nil, nil
		}

		// run second callback
		enforcers2, err := b.Handler(ctx)
		if err != nil {
			return nil, err
		} else if len(enforcers2) == 0 {
			return nil, nil
		}

		// merge both sets
		enforcers := make(S, 0, len(enforcers1)+len(enforcers2))
		enforcers = append(enforcers, enforcers1...)
		enforcers = append(enforcers, enforcers2...)

		return enforcers, nil
	})
}

// And will run And() with the current and specified authorizer.
func (a *Authorizer) And(b *Authorizer) *Authorizer {
	return And(a, b)
}

// Or will match and run the first authorizer and return its enforcers on success.
// If no enforcers are returned it will match and run the second authorizer and
// return its enforcers on success.
func Or(a, b *Authorizer) *Authorizer {
	return A("ash/Or", func(ctx *fire.Context) bool {
		return a.Matcher(ctx) || b.Matcher(ctx)
	}, func(ctx *fire.Context) ([]*Enforcer, error) {
		// check first authorizer
		if a.Matcher(ctx) {
			// run callback
			enforcers, err := a.Handler(ctx)
			if err != nil {
				return nil, err
			}

			// return on success
			if len(enforcers) > 0 {
				return enforcers, nil
			}
		}

		// check second authorizer
		if b.Matcher(ctx) {
			// run callback
			enforcers, err := b.Handler(ctx)
			if err != nil {
				return nil, err
			}

			// return on success
			if len(enforcers) > 0 {
				return enforcers, nil
			}
		}

		return nil, nil
	})
}

// Or will run Or() with the current and specified authorizer.
func (a *Authorizer) Or(b *Authorizer) *Authorizer {
	return Or(a, b)
}
