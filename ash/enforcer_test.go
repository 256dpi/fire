package ash

import (
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestGrantAccess(t *testing.T) {
	assert.NoError(t, tester.RunCallback(nil, GrantAccess()))
}

func TestDenyAccess(t *testing.T) {
	assert.Equal(t, fire.ErrAccessDenied, tester.RunCallback(nil, DenyAccess()))
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
