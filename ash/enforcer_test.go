package ash

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
)

func TestGrantAccess(t *testing.T) {
	assert.NoError(t, tester.RunCallback(nil, GrantAccess()))
}

func TestDenyAccess(t *testing.T) {
	err := tester.RunCallback(nil, DenyAccess())
	assert.True(t, errors.Is(err, fire.ErrAccessDenied))
}

func TestAddFilter(t *testing.T) {
	ctx := &fire.Context{}

	err := tester.RunCallback(ctx, AddFilter(bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, []bson.M{{"foo": "bar"}}, ctx.Filters)
}

func TestWhitelistReadableFields(t *testing.T) {
	ctx := &fire.Context{ReadableFields: []string{"foo", "bar", "baz"}}
	err := tester.RunCallback(ctx, WhitelistReadableFields("bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar"}, ctx.ReadableFields)

	ctx = &fire.Context{ReadableFields: []string{}}
	err = tester.RunCallback(ctx, WhitelistReadableFields("foo", "bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{}, ctx.ReadableFields)
}

func TestWhitelistWritableFields(t *testing.T) {
	ctx := &fire.Context{WritableFields: []string{"foo", "bar", "baz"}}
	err := tester.RunCallback(ctx, WhitelistWritableFields("bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar"}, ctx.WritableFields)

	ctx = &fire.Context{WritableFields: []string{}}
	err = tester.RunCallback(ctx, WhitelistWritableFields("foo", "bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{}, ctx.WritableFields)
}

func TestAddRelationshipFilter(t *testing.T) {
	ctx := &fire.Context{}

	err := tester.RunCallback(ctx, AddRelationshipFilter("foo", bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, map[string][]bson.M{"foo": {{"foo": "bar"}}}, ctx.RelationshipFilters)
}
