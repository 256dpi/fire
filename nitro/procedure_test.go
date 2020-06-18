package nitro

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

type jsonProcedure struct {
	Base `json:"-" nitro:"test"`

	Foo string `json:"foo"`
}

func (t *jsonProcedure) Validate() error {
	// check foo
	if t.Foo == "" {
		return fmt.Errorf("missing foo")
	}

	return nil
}

type bsonProcedure struct {
	Base `bson:"-" nitro:"test"`

	Foo string
}

func (t *bsonProcedure) Validate() error {
	// check foo
	if t.Foo == "" {
		return fmt.Errorf("missing foo")
	}

	return nil
}

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&jsonProcedure{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(jsonProcedure{}),
		Name:   "test",
		Coding: stick.JSON,
		Accessor: &stick.Accessor{
			Name: "nitro.jsonProcedure",
			Fields: map[string]*stick.Field{
				"Foo": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	meta = GetMeta(&bsonProcedure{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(bsonProcedure{}),
		Name:   "test",
		Coding: stick.BSON,
		Accessor: &stick.Accessor{
			Name: "nitro.bsonProcedure",
			Fields: map[string]*stick.Field{
				"Foo": {
					Index: 1,
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
	proc := &jsonProcedure{
		Foo: "foo",
	}

	val, ok := stick.Get(proc, "foo")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(proc, "Foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", val)

	ok = stick.Set(proc, "foo", "bar")
	assert.False(t, ok)
	assert.Equal(t, "foo", proc.Foo)

	ok = stick.Set(proc, "Foo", "bar")
	assert.True(t, ok)
	assert.Equal(t, "bar", proc.Foo)
}
