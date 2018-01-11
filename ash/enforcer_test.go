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

func TestFilterQuery(t *testing.T) {
	filter := bson.M{}
	err := tester.RunCallback(&fire.Context{Filter: filter}, FilterQuery(bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "bar", filter["foo"])
}
