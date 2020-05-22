package cinder

import (
	"context"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestGetSpan(t *testing.T) {
	span := GetSpan(nil)
	assert.Nil(t, span)

	root, ctx := opentracing.StartSpanFromContext(context.Background(), "root")

	span = GetSpan(ctx)
	assert.Equal(t, root, span)

	trace, ctx := CreateTrace(ctx, "trace")

	span = GetSpan(ctx)
	assert.NotEqual(t, root, span)
	assert.Equal(t, trace.Root(), span)

	trace.Push("foo")

	span = GetSpan(ctx)
	assert.Equal(t, trace.Tail(), span)
}

func TestBranch(t *testing.T) {
	branch := Branch(nil)
	assert.Nil(t, branch)

	root, ctx := opentracing.StartSpanFromContext(context.Background(), "root")

	branch = Branch(ctx)
	assert.Equal(t, root, opentracing.SpanFromContext(branch))

	trace, ctx := CreateTrace(ctx, "trace")

	branch = Branch(ctx)
	assert.Equal(t, trace.Root(), opentracing.SpanFromContext(branch))

	trace.Push("foo")

	branch = Branch(ctx)
	assert.Equal(t, trace.Tail(), opentracing.SpanFromContext(branch))
}
