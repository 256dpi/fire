package nitro

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

type testProcedure struct {
	Base `json:"-" nitro:"test"`

	User string `json:"user"`
	Role string `json:"role"`
}

func (t *testProcedure) Validate() error {
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

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&testProcedure{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(testProcedure{}),
		URL:    "test",
		Coding: stick.JSON,
		Accessor: &stick.Accessor{
			Name: "nitro.testProcedure",
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

	assert.PanicsWithValue(t, `nitro: expected first struct field to be an embedded "nitro.Base"`, func() {
		type invalidProcedure struct {
			Hello string
			Base
			stick.NoValidation
		}

		GetMeta(&invalidProcedure{})
	})

	assert.PanicsWithValue(t, `nitro: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "nitro.Base"`, func() {
		type invalidProcedure struct {
			Base  `nitro:"foo"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidProcedure{})
	})

	assert.PanicsWithValue(t, `nitro: expected to find a tag of the form 'nitro:"name"' on "nitro.Base"`, func() {
		type invalidProcedure struct {
			Base  `json:"-" nitro:""`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidProcedure{})
	})
}

func TestDynamicAccess(t *testing.T) {
	proc := &testProcedure{
		User: "user",
	}

	val, ok := stick.Get(proc, "user")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(proc, "User")
	assert.True(t, ok)
	assert.Equal(t, "user", val)

	ok = stick.Set(proc, "user", "foo")
	assert.False(t, ok)
	assert.Equal(t, "user", proc.User)

	ok = stick.Set(proc, "User", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", proc.User)
}
