package fire

import (
	"context"
	"net/http"

	"github.com/opentracing/opentracing-go"
)

// Tracer provides a simple wrapper around the opentracing API to instrument
// and trace code.
type Tracer struct {
	root  opentracing.Span
	spans []opentracing.Span
}

// NewTracerFromRequest returns a new tracer that has a root span derive from
// the specified request.
func NewTracerFromRequest(r *http.Request, name string) *Tracer {
	// create from request
	span, _ := opentracing.StartSpanFromContext(r.Context(), name)
	span = span.SetTag("http.method", r.Method)
	span = span.SetTag("http.url", r.URL.Path)
	span = span.SetTag("peer.address", r.RemoteAddr)

	return NewTracer(span)
}

// NewTracerWithRoot returns a new tracer that has a root span created with the
// specified name.
func NewTracerWithRoot(name string) *Tracer {
	return NewTracer(opentracing.StartSpan(name))
}

// NewTracer returns a new tracer with the specified root span.
func NewTracer(root opentracing.Span) *Tracer {
	return &Tracer{
		root:  root,
		spans: make([]opentracing.Span, 0, 32),
	}
}

// Push will add a new span on to the stack. Successful spans must be finished by
// calling Pop. If the code panics or an error is returned the last pushed span
// will be flagged with the error and a leftover spans are popped.
func (t *Tracer) Push(name string) {
	// get context
	var ctx opentracing.SpanContext
	if len(t.spans) > 0 {
		ctx = t.Last().Context()
	} else {
		ctx = t.root.Context()
	}

	// create new span
	span := opentracing.StartSpan(name, opentracing.ChildOf(ctx))

	// push span
	t.spans = append(t.spans, span)
}

// Last returns the last pushed span or the root span.
func (t *Tracer) Last() opentracing.Span {
	// return root if empty
	if len(t.spans) == 0 {
		return t.root
	}

	return t.spans[len(t.spans)-1]
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
// handlers that will call NewTracerFromRequest or similar.
func (t *Tracer) Context(ctx context.Context) context.Context {
	return opentracing.ContextWithSpan(ctx, t.Last())
}

// Pop finishes and removes the last pushed span.
func (t *Tracer) Pop() {
	// check list
	if len(t.spans) == 0 {
		return
	}

	// finish last span
	t.Last().Finish()

	// resize slice
	t.spans = t.spans[:len(t.spans)-1]
}

// Finish will finish all leftover spans and the root span if requested.
func (t *Tracer) Finish(root bool) {
	for _, span := range t.spans {
		span.Finish()
	}

	if root {
		t.root.Finish()
	}
}
