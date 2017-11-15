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

	ctx := context(fire.Find)
	err := QueryFilter(bson.M{
		"foo": "bar",
	})(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "bar", ctx.Query["foo"])

	ctx = context(fire.Find)
	err = HideFilter()(ctx)
	assert.NoError(t, err)
	assert.Len(t, ctx.Query, 1)

	assert.Panics(t, func() {
		HideFilter()(context(fire.Create))
	})
}
