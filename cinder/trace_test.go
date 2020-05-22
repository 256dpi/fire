package cinder

import (
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestTrace(t *testing.T) {
	trace, ctx := CreateTrace(nil, "trace")
	assert.NotNil(t, trace)
	assert.Equal(t, trace, GetTrace(ctx))
	assert.Equal(t, trace.Root(), opentracing.SpanFromContext(ctx))
	assert.Equal(t, trace.Tail(), opentracing.SpanFromContext(ctx))

	trace.Push("foo")
	assert.Equal(t, trace.Root(), opentracing.SpanFromContext(ctx))
	assert.NotEqual(t, trace.Tail(), opentracing.SpanFromContext(ctx))

	trace.Pop()
	assert.Equal(t, trace.Root(), opentracing.SpanFromContext(ctx))
	assert.Equal(t, trace.Tail(), opentracing.SpanFromContext(ctx))
}
