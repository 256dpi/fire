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

func TestWhitelistFields(t *testing.T) {
	ctx := &fire.Context{Fields: nil}
	err := tester.RunCallback(ctx, WhitelistFields("foo", "bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"foo", "bar"}, ctx.Fields)

	ctx = &fire.Context{Fields: []string{"foo", "bar", "baz"}}
	err = tester.RunCallback(ctx, WhitelistFields("bar"))
	assert.NoError(t, err)
	assert.Equal(t, []string{"bar"}, ctx.Fields)
}
