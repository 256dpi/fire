package heat

import (
	"reflect"
	"testing"
	"time"

	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

type testKey struct {
	Base `json:"-" heat:"test,1h"`

	User string `json:"user"`
	Role string `json:"role"`
}

func (t *testKey) Validate() error {
	// check user
	if t.User == "" {
		return xo.F("missing user")
	}

	// check role
	if t.Role == "" {
		return xo.F("missing role")
	}

	return nil
}

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&testKey{})
	assert.Equal(t, &Meta{
		Name:   "test",
		Expiry: time.Hour,
		Accessor: &stick.Accessor{
			Name: "heat.testKey",
			Fields: map[string]*stick.Field{
				"User": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
				"Role": {
					Index: 2,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	assert.PanicsWithValue(t, `heat: expected first struct field to be an embedded "heat.Base"`, func() {
		type invalidKey struct {
			Hello string
			Base
			stick.NoValidation
		}

		GetMeta(&invalidKey{})
	})

	assert.PanicsWithValue(t, `heat: expected to find a tag of the form 'json:"-"' on "heat.Base"`, func() {
		type invalidKey struct {
			Base  `heat:"foo,1h"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidKey{})
	})

	assert.PanicsWithValue(t, `heat: expected to find a tag of the form 'heat:"name,expiry"' on "heat.Base"`, func() {
		type invalidKey struct {
			Base  `json:"-" heat:","`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidKey{})
	})

	assert.PanicsWithValue(t, `heat: invalid duration as expiry on "heat.Base"`, func() {
		type invalidKey struct {
			Base  `json:"-" heat:"foo,bar"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidKey{})
	})
}

func TestDynamicAccess(t *testing.T) {
	key := &testKey{
		User: "user",
	}

	val, ok := stick.Get(key, "user")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(key, "User")
	assert.True(t, ok)
	assert.Equal(t, "user", val)

	ok = stick.Set(key, "user", "foo")
	assert.False(t, ok)
	assert.Equal(t, "user", key.User)

	ok = stick.Set(key, "User", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", key.User)
}
