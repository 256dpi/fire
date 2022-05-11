package ash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestGrantAccess(t *testing.T) {
	assert.NoError(t, tester.RunCallback(nil, GrantAccess()))
}

func TestDenyAccess(t *testing.T) {
	err := tester.RunCallback(nil, DenyAccess())
	assert.True(t, fire.ErrAccessDenied.Is(err))
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

func TestSetReadableFieldsGetter(t *testing.T) {
	ctx := &fire.Context{}
	err := tester.RunCallback(ctx, SetReadableFieldsGetter(func(ctx *fire.Context, model coal.Model) []string {
		return nil
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ctx.GetReadableFields)
	err = tester.RunCallback(ctx, SetReadableFieldsGetter(func(ctx *fire.Context, model coal.Model) []string {
		return nil
	}))
	assert.Error(t, err)
	assert.Equal(t, "existing readable fields getter", err.Error())
}

func TestWhitelistWritableFields(t *testing.T) {
	ctx := &fire.Context{Operation: fire.Create, WritableFields: []string{"foo", "bar", "baz"}}
	err := tester.RunCallback(ctx, WhitelistWritableFields("bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar"}, ctx.WritableFields)

	ctx = &fire.Context{Operation: fire.Create, WritableFields: []string{}}
	err = tester.RunCallback(ctx, WhitelistWritableFields("foo", "bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{}, ctx.WritableFields)
}

func TestSetWritableFieldsGetter(t *testing.T) {
	ctx := &fire.Context{Operation: fire.Create}
	err := tester.RunCallback(ctx, SetWritableFieldsGetter(func(ctx *fire.Context, model coal.Model) []string {
		return nil
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ctx.GetWritableFields)
	err = tester.RunCallback(ctx, SetWritableFieldsGetter(func(ctx *fire.Context, model coal.Model) []string {
		return nil
	}))
	assert.Error(t, err)
	assert.Equal(t, "existing writable fields getter", err.Error())
}

func TestWhitelistReadableProperties(t *testing.T) {
	ctx := &fire.Context{ReadableProperties: []string{"foo", "bar", "baz"}}
	err := tester.RunCallback(ctx, WhitelistReadableProperties("bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar"}, ctx.ReadableProperties)

	ctx = &fire.Context{ReadableProperties: []string{}}
	err = tester.RunCallback(ctx, WhitelistReadableProperties("foo", "bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{}, ctx.ReadableProperties)
}

func TestSetReadablePropertiesGetter(t *testing.T) {
	ctx := &fire.Context{}
	err := tester.RunCallback(ctx, SetReadablePropertiesGetter(func(ctx *fire.Context, model coal.Model) []string {
		return nil
	}))
	assert.NoError(t, err)
	assert.NotNil(t, ctx.GetReadableProperties)
	err = tester.RunCallback(ctx, SetReadablePropertiesGetter(func(ctx *fire.Context, model coal.Model) []string {
		return nil
	}))
	assert.Error(t, err)
	assert.Equal(t, "existing readable properties getter", err.Error())
}

func TestAddRelationshipFilter(t *testing.T) {
	ctx := &fire.Context{}

	err := tester.RunCallback(ctx, AddRelationshipFilter("foo", bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, map[string][]bson.M{"foo": {{"foo": "bar"}}}, ctx.RelationshipFilters)
}
