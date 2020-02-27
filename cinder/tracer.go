package cinder

import (
	"context"
	"net/http"

	"github.com/opentracing/opentracing-go"
)

// Tracer provides a simple wrapper around the opentracing API to instrument
// and trace code.
type Tracer struct {
	root  opentracing.Span
	stack []opentracing.Span
}

// TraceRequest returns a new tracer that has a root span derived from the
// specified request. A span previously added to the request context using
// Context is automatically used as the parent.
func TraceRequest(r *http.Request, name string) *Tracer {
	span, _ := opentracing.StartSpanFromContext(r.Context(), name)
	return Use(span)
}

// Trace returns a new tracer that has a root span created with the specified
// name.
func Trace(name string) *Tracer {
	return Use(opentracing.StartSpan(name))
}

// Use returns a new tracer that uses the provided span as its root.
func Use(span opentracing.Span) *Tracer {
	return &Tracer{
		root:  span,
		stack: make([]opentracing.Span, 0, 32),
	}
}

// Push will add a new span on to the stack. Successful spans must be finished by
// calling Pop. If the code panics or an error is returned the last pushed span
// will be flagged with the error and all leftover spans are popped.
func (t *Tracer) Push(name string) {
	// get context
	var ctx opentracing.SpanContext
	if len(t.stack) > 0 {
		ctx = t.Last().Context()
	} else {
		ctx = t.root.Context()
	}

	// create new span
	span := opentracing.StartSpan(name, opentracing.ChildOf(ctx))

	// push span
	t.stack = append(t.stack, span)
}

// Last returns the last pushed span or the root span.
func (t *Tracer) Last() opentracing.Span {
	// return root if empty
	if len(t.stack) == 0 {
		return t.root
	}

	return t.stack[len(t.stack)-1]
}

// Tag adds a tag to the last pushed span.
func (t *Tracer) Tag(key string, value interface{}) {
	t.Last().SetTag(key, value)
}

// Log adds a log to the last pushed span.
func (t *Tracer) Log(key string, value interface{}) {
	t.Last().LogKV(key, value)
}

// Context returns a new context with the latest span stored as a reference for
// handlers that will call TraceRequest or similar.
func (t *Tracer) Context(ctx context.Context) context.Context {
	return opentracing.ContextWithSpan(ctx, t.Last())
}

// Pop finishes and removes the last pushed span.
func (t *Tracer) Pop() {
	// check list
	if len(t.stack) == 0 {
		return
	}

	// finish last span
	t.Last().Finish()

	// resize slice
	t.stack = t.stack[:len(t.stack)-1]
}

// Finish will finish all leftover spans and the root span if requested.
func (t *Tracer) Finish(root bool) {
	// finish all stacked spans
	for _, span := range t.stack {
		span.Finish()
	}

	// finish root span if requested
	if root {
		t.root.Finish()
	}
}
