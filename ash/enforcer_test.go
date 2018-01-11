package ash

import (
	"testing"

	"github.com/256dpi/fire"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestEnforcer(t *testing.T) {
	assert.NoError(t, tester.RunAuthorizer(fire.List, nil, nil, nil, GrantAccess()))
	assert.Equal(t, fire.ErrAccessDenied, tester.RunAuthorizer(fire.List, nil, nil, nil, DenyAccess()))

	filter := bson.M{}
	err := tester.RunAuthorizer(fire.List, nil, filter, nil, AddFilter(bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "bar", filter["foo"])
}
