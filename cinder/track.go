package cinder

import (
	"context"

	"github.com/opentracing/opentracing-go"
)

// Span is the underlying span used for tracing.
type Span struct {
	span opentracing.Span
}

// Track is used to mark and annotate a function call. It will automatically
// wrap the context with a child from the span history found in the provided
// context. If no span history was found it will return a noop span.
//
// If the function finds a trace in the context and its root span matches
// the span from the context it will create a child from the traces tail.
// If not it considers the span history to have diverged from the trace.
func Track(ctx context.Context, name string) (context.Context, *Span) {
	// get span
	span := GetSpan(ctx)
	if span == nil {
		return ctx, nil
	}

	// create child
	span = opentracing.StartSpan(name, opentracing.ChildOf(span.Context()))

	// wrap context
	ctx = opentracing.ContextWithSpan(ctx, span)

	return ctx, &Span{span: span}
}

// Tag will tag the span.
func (s *Span) Tag(key string, value interface{}) {
	if s != nil && s.span != nil {
		s.span.SetTag(key, value)
	}
}

// Log will log to the span.
func (s *Span) Log(key string, value interface{}) {
	if s != nil && s.span != nil {
		s.span.LogKV(key, value)
	}
}

// Finish will finish the span.
func (s *Span) Finish() {
	if s != nil && s.span != nil {
		s.span.Finish()
	}
}

// Native will return the underlying native span.
func (s *Span) Native() opentracing.Span {
	if s != nil {
		return s.span
	}

	return nil
}
