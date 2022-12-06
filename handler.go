package fire

import (
	"time"

	"github.com/256dpi/xo"
)

// A Callback is called during the request processing flow of a controller.
type Callback struct {
	// The name.
	Name string

	// The stage.
	Stage Stage

	// The matcher that decides whether the callback should be run.
	Matcher Matcher

	// The handler that gets executed with the context.
	//
	// If returned errors are marked with Safe() they will be included in the
	// returned JSON-API error.
	Handler Handler
}

// L is a shorthand type to create a list of callbacks.
type L = []*Callback

// C is a shorthand function to construct a callback. It will also add tracing
// code around the execution of the callback.
func C(name string, s Stage, m Matcher, h Handler) *Callback {
	// panic if parameters are not set
	if name == "" || m == nil || h == nil {
		panic("fire: missing parameters")
	}

	return &Callback{
		Name:    name,
		Stage:   s,
		Matcher: m,
		Handler: func(ctx *Context) error {
			// trace
			ctx.Tracer.Push(name)
			defer ctx.Tracer.Pop()

			// call handler
			err := h(ctx)
			if err != nil {
				return xo.W(err)
			}

			return nil
		},
	}
}

// An Action defines a collection or resource action.
type Action struct {
	// The allowed methods for this action.
	Methods []string

	// BodyLimit defines the maximum allowed size of the request body. The
	// serve.ByteSize helper can be used to set the value.
	//
	// Default: 8M.
	BodyLimit int64

	// Timeout defines the time after which the context is cancelled and
	// processing of the action should be stopped.
	//
	// Default: 30s.
	Timeout time.Duration

	// The handler that gets executed with the context.
	//
	// If returned errors are marked with Safe() they will be included in the
	// returned JSON-API error.
	Handler Handler
}

// M is a shorthand type to create a map of actions.
type M = map[string]*Action

// A is a shorthand function to construct an action.
func A(name string, methods []string, bodyLimit int64, timeout time.Duration, h Handler) *Action {
	// panic if methods or handler is not set
	if len(methods) == 0 || h == nil {
		panic("fire: missing methods or handler")
	}

	return &Action{
		Methods:   methods,
		BodyLimit: bodyLimit,
		Timeout:   timeout,
		Handler: func(ctx *Context) error {
			// trace
			ctx.Tracer.Push(name)
			defer ctx.Tracer.Pop()

			// call handler
			err := h(ctx)
			if err != nil {
				return xo.W(err)
			}

			return nil
		},
	}
}

// Handler is function that takes a context, mutates is to modify the behaviour
// and response or return an error.
type Handler func(ctx *Context) error

// Matcher is a function that makes an assessment of a context and decides whether
// an operation should be allowed to be carried out.
type Matcher func(ctx *Context) bool

// All will match all contexts.
func All() Matcher {
	return func(ctx *Context) bool {
		return true
	}
}

// Only will match if the operation is present in the provided list.
func Only(op Operation) Matcher {
	return func(ctx *Context) bool {
		return ctx.Operation&op != 0
	}
}

// Except will match if the operation is not present in the provided list.
func Except(op Operation) Matcher {
	return func(ctx *Context) bool {
		return ctx.Operation&op == 0
	}
}

// Combine will combine multiple callbacks.
func Combine(name string, stage Stage, cbs ...*Callback) *Callback {
	// check stage
	if stage != 0 {
		for _, cb := range cbs {
			if cb.Stage&stage == 0 {
				panic("fire: callback does not support stage")
			}
		}
	}

	return C(name, stage, func(ctx *Context) bool {
		// check if one of the callback matches
		for _, cb := range cbs {
			if cb.Matcher(ctx) {
				return true
			}
		}

		return false
	}, func(ctx *Context) error {
		// call all matching callbacks
		for _, cb := range cbs {
			if ctx.Stage&cb.Stage != 0 && cb.Matcher(ctx) {
				err := cb.Handler(ctx)
				if err != nil {
					return xo.W(err)
				}
			}
		}

		return nil
	})
}
