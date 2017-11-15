package ash

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestEnforcer(t *testing.T) {
	assert.NoError(t, AccessGranted()(nil))
	assert.Equal(t, errAccessDenied, AccessDenied()(nil))

	query := bson.M{}
	err := tester.RunAuthorizer(fire.List, query, nil, QueryFilter(bson.M{
		"foo": "bar",
	}))
	assert.NoError(t, err)
	assert.Equal(t, "bar", query["foo"])

	query = bson.M{}
	err = tester.RunAuthorizer(fire.Find, query, nil, HideFilter())
	assert.NoError(t, err)
	assert.Len(t, query, 1)

	assert.Panics(t, func() {
		tester.RunAuthorizer(fire.Create, query, nil, HideFilter())
	})
}
