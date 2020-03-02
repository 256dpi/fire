// Package cinder provides instrumentation capabilities to applications and
// components.
package cinder

import (
	"context"

	"github.com/opentracing/opentracing-go"
)

type traceContextKey struct{}

var traceKey = traceContextKey{}

// Trace implements a span stack that can be used with fat contexts. Rather than
// branching of the context for every function call, a span is pushed onto the
// stack to track execution.
type Trace struct {
	root  opentracing.Span
	stack []opentracing.Span
}

// CreateTrace returns a new trace that will use the span found in the provided
// context as its root or start a new one. The returned context is the provided
// context wrapped with the new span and trace.
func CreateTrace(ctx context.Context, name string) (*Trace, context.Context) {
	// check context
	if ctx == nil {
		ctx = context.Background()
	}

	// get span
	span, ctx := opentracing.StartSpanFromContext(ctx, name)

	// create trace
	trace := &Trace{
		root:  span,
		stack: make([]opentracing.Span, 0, 32),
	}

	// add trace
	ctx = context.WithValue(ctx, traceKey, trace)

	return trace, ctx
}

// GetTrace will return the trace from the context or nil if absent.
func GetTrace(ctx context.Context) *Trace {
	// check context
	if ctx == nil {
		return nil
	}

	// get trace
	trace, _ := ctx.Value(traceKey).(*Trace)

	return trace
}

// Push will add a new span to the trace.
func (t *Trace) Push(name string) {
	// get parent
	parent := t.Tail().Context()

	// create child
	child := opentracing.StartSpan(name, opentracing.ChildOf(parent))

	// push child
	t.stack = append(t.stack, child)
}

// Tag adds a tag to the last pushed span.
func (t *Trace) Tag(key string, value interface{}) {
	t.Tail().SetTag(key, value)
}

// Log adds a log to the last pushed span.
func (t *Trace) Log(key string, value interface{}) {
	t.Tail().LogKV(key, value)
}

// Pop finishes and removes the last pushed span. This call is usually deferred
// right after a push.
func (t *Trace) Pop() {
	// check list
	if len(t.stack) == 0 {
		return
	}

	// finish last span
	t.stack[len(t.stack)-1].Finish()

	// resize stack
	t.stack = t.stack[:len(t.stack)-1]
}

// Finish will finish the root and all stacked spans.
func (t *Trace) Finish() {
	// finish stacked spans
	for _, span := range t.stack {
		span.Finish()
	}

	// finish root span
	t.root.Finish()
}

// Tail returns the tail or root of the span stack.
func (t *Trace) Tail() opentracing.Span {
	// return last span if available
	if len(t.stack) > 0 {
		return t.stack[len(t.stack)-1]
	}

	return t.root
}

// Root will return the root of the span stack.
func (t *Trace) Root() opentracing.Span {
	return t.root
}
