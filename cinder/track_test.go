package cinder

import (
	"context"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestTrack(t *testing.T) {
	ctx, span := Track(nil, "foo")
	assert.Nil(t, ctx)
	assert.Nil(t, span)

	ctx, span = Track(context.Background(), "foo")
	assert.NotNil(t, ctx)
	assert.Nil(t, span)

	_, root := opentracing.StartSpanFromContext(context.Background(), "root")

	ctx, span = Track(root, "track")
	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
}
