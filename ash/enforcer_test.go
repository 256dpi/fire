package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestEnforcer(t *testing.T) {
	assert.NoError(t, GrantAccess().Handler(nil))
	assert.Equal(t, ErrAccessDenied, DenyAccess().Handler(nil))

	filter := bson.M{}
	err := tester.RunAuthorizer(fire.List, nil, filter, nil, AddFilter(bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "bar", filter["foo"])

	filter = bson.M{}
	err = tester.RunAuthorizer(fire.Find, nil, filter, nil, HideFilter())
	assert.NoError(t, err)
	assert.Len(t, filter, 1)

	assert.Panics(t, func() {
		tester.RunAuthorizer(fire.Create, nil, filter, nil, HideFilter())
	})
}
