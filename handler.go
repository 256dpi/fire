package fire

// A Callback is called during the request processing flow of a controller.
type Callback struct {
	// The matcher that decides whether the callback should be run.
	Matcher Matcher

	// The handler handler that gets executed with the context.
	//
	// If returned errors are marked with Safe() they will be included in the
	// returned JSON-API error.
	Handler Handler
}

// L is a short-hand type to create a list of callbacks.
type L []*Callback

// C is a short-hand function to construct a callback. It will also add tracing
// code around the execution of the callback.
func C(name string, m Matcher, h Handler) *Callback {
	// panic if matcher or handler is not set
	if m == nil || h == nil {
		panic("fire: missing matcher or handler")
	}

	return &Callback{
		Matcher: m,
		Handler: func(ctx *Context) error {
			// begin trace
			ctx.Tracer.Push(name)

			// call handler
			err := h(ctx)
			if err != nil {
				return err
			}

			// finish trace
			ctx.Tracer.Pop()

			return nil
		},
	}
}

// An Action defines a collection or resource action.
type Action struct {
	// The allowed methods for this action.
	Methods []string

	// The callback for this action.
	Callback *Callback

	// BodyLimit defines the maximum allowed size of the request body. It
	// defaults to 8M if set to zero. The DataSize helper can be used to set
	// the value.
	BodyLimit uint64
}

// M is a short-hand type to create a map of actions.
type M map[string]*Action

// A is a short-hand function to construct an action.
func A(name string, methods []string, h Handler) *Action {
	return &Action{
		Methods:  methods,
		Callback: C(name, All(), h),
	}
}

// Handler is function that takes a context, mutates is to modify the behaviour
// and response or return an error.
type Handler func(*Context) error

// Matcher is a function that makes an assessment of a context and decides whether
// a modification should be applied in the future.
type Matcher func(*Context) bool

// All will match all contexts.
func All() Matcher {
	return func(ctx *Context) bool {
		return true
	}
}

// Only will match if the operation is present in the provided list.
func Only(ops ...Operation) Matcher {
	return func(ctx *Context) bool {
		// allow if operation is listed
		for _, op := range ops {
			if op == ctx.Operation {
				return true
			}
		}

		return false
	}
}

// Except will match if the operation is not present in the provided list.
func Except(ops ...Operation) Matcher {
	return func(ctx *Context) bool {
		// disallow if operation is listed
		for _, op := range ops {
			if op == ctx.Operation {
				return false
			}
		}

		return true
	}
}
