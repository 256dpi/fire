package heat

import (
	"fmt"
	"reflect"
	"testing"
	"time"

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
		return fmt.Errorf("missing user")
	}

	// check role
	if t.Role == "" {
		return fmt.Errorf("missing role")
	}

	return nil
}

type invalidKey1 struct {
	Hello string
	Base
	stick.NoValidation
}

type invalidKey2 struct {
	Base  `heat:"foo,1h"`
	Hello string
	stick.NoValidation
}

type invalidKey3 struct {
	Base  `json:"-" heat:","`
	Hello string
	stick.NoValidation
}

type invalidKey4 struct {
	Base  `json:"-" heat:"foo,bar"`
	Hello string
	stick.NoValidation
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
		GetMeta(&invalidKey1{})
	})

	assert.PanicsWithValue(t, `heat: expected to find a tag of the form 'json:"-"' on "heat.Base"`, func() {
		GetMeta(&invalidKey2{})
	})

	assert.PanicsWithValue(t, `heat: expected to find a tag of the form 'heat:"name,expiry"' on "heat.Base"`, func() {
		GetMeta(&invalidKey3{})
	})

	assert.PanicsWithValue(t, `heat: invalid duration as expiry on "heat.Base"`, func() {
		GetMeta(&invalidKey4{})
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
