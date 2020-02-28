// Package cinder provides instrumentation capabilities to fire applications and
// components.
package cinder

import (
	"context"

	"github.com/opentracing/opentracing-go"
)

// Trace provides a simple wrapper around opentracing to instrument and trace
// code execution.
type Trace struct {
	root  opentracing.Span
	stack []opentracing.Span
}

// New returns a new trace that will use the span found in the provided context
// as its base or start a new one.
func New(ctx context.Context, name string) *Trace {
	// check context
	if ctx == nil {
		ctx = context.Background()
	}

	// get span
	span, _ := opentracing.StartSpanFromContext(ctx, name)

	return &Trace{
		root:  span,
		stack: make([]opentracing.Span, 0, 32),
	}
}

// Push will add a new span to the stack.
func (t *Trace) Push(name string) {
	// get context
	var ctx opentracing.SpanContext
	if len(t.stack) > 0 {
		ctx = t.Tail().Context()
	} else {
		ctx = t.root.Context()
	}

	// create new span
	span := opentracing.StartSpan(name, opentracing.ChildOf(ctx))

	// push span
	t.stack = append(t.stack, span)
}

// Tag adds a tag to the last pushed span.
func (t *Trace) Tag(key string, value interface{}) {
	t.Tail().SetTag(key, value)
}

// Log adds a log to the last pushed span.
func (t *Trace) Log(key string, value interface{}) {
	t.Tail().LogKV(key, value)
}

// Pop finishes and removes the last pushed span. In function calls that return
// errors the Pop can simply be deferred after Push. In functions that use panic
// and recover, Pop should be called before all returns without defer to allow
// the panic handler to add the error to the proper span.
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

// Finish will finish all leftover spans and the root span if requested.
func (t *Trace) Finish() {
	// finish all stacked spans
	for _, span := range t.stack {
		span.Finish()
	}

	// finish root span
	t.root.Finish()
}

// Tail returns the last pushed span or the root span.
func (t *Trace) Tail() opentracing.Span {
	// return last span if available
	if len(t.stack) > 0 {
		return t.stack[len(t.stack)-1]
	}

	return t.root
}

// Wrap will wrap the provided context with the current tail span to allow
// interoperability with other libraries.
func (t *Trace) Wrap(ctx context.Context) context.Context {
	return opentracing.ContextWithSpan(ctx, t.Tail())
}
