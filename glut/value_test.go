package glut

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

type bsonValue struct {
	Base `bson:"-" glut:"bson,0"`
	Data string `bson:"data"`
}

func (v *bsonValue) Validate() error {
	return nil
}

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&testValue{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(&testValue{}),
		Key:    "test",
		TTL:    0,
		Coding: stick.JSON,
		Accessor: &stick.Accessor{
			Name: "glut.testValue",
			Fields: map[string]*stick.Field{
				"Data": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	meta = GetMeta(&bsonValue{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(&bsonValue{}),
		Key:    "bson",
		TTL:    0,
		Coding: stick.BSON,
		Accessor: &stick.Accessor{
			Name: "glut.bsonValue",
			Fields: map[string]*stick.Field{
				"Data": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	assert.PanicsWithValue(t, `glut: expected first struct field to be an embedded "glut.Base"`, func() {
		type invalidValue struct {
			Hello string
			Base
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})

	assert.PanicsWithValue(t, `glut: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "glut.Base"`, func() {
		type invalidValue struct {
			Base  `glut:"foo/bar"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})

	assert.PanicsWithValue(t, `glut: expected to find a tag of the form 'glut:"key,ttl"' on "glut.Base"`, func() {
		type invalidValue struct {
			Base  `json:"-" glut:""`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})

	assert.PanicsWithValue(t, `glut: invalid duration as time to live on "glut.Base"`, func() {
		type invalidValue struct {
			Base  `json:"-" glut:"foo,bar"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})
}

func TestDynamicAccess(t *testing.T) {
	value := &testValue{
		Data: "data",
	}

	val, ok := stick.Get(value, "data")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(value, "Data")
	assert.True(t, ok)
	assert.Equal(t, "data", val)

	ok = stick.Set(value, "data", "foo")
	assert.False(t, ok)
	assert.Equal(t, "data", value.Data)

	ok = stick.Set(value, "Data", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", value.Data)
}
