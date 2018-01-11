package ash

import (
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestEnforcer(t *testing.T) {
	assert.NoError(t, tester.RunCallback(nil, GrantAccess()))
	assert.Equal(t, fire.ErrAccessDenied, tester.RunCallback(nil, DenyAccess()))

	filter := bson.M{}
	err := tester.RunCallback(&fire.Context{Filter: filter}, FilterQuery(bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "bar", filter["foo"])
}
