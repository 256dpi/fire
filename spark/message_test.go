package spark

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

type testMessage struct {
	Base `json:"-" spark:"test"`

	Command string `json:"command"`
}

func (t *testMessage) Validate() error {
	// check command
	if t.Command == "" {
		return fmt.Errorf("missing command")
	}

	return nil
}

type bsonMessage struct {
	Base `bson:"-" spark:"bson"`
	Data string `bson:"data"`
}

func (v *bsonMessage) Validate() error {
	return nil
}

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&testMessage{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(&testMessage{}),
		Name:   "test",
		Coding: stick.JSON,
		Accessor: &stick.Accessor{
			Name: "spark.testMessage",
			Fields: map[string]*stick.Field{
				"Command": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	meta = GetMeta(&bsonMessage{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(&bsonMessage{}),
		Name:   "bson",
		Coding: stick.BSON,
		Accessor: &stick.Accessor{
			Name: "spark.bsonMessage",
			Fields: map[string]*stick.Field{
				"Data": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	assert.PanicsWithValue(t, `spark: expected first struct field to be an embedded "spark.Base"`, func() {
		type invalidValue struct {
			Hello string
			Base
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})

	assert.PanicsWithValue(t, `spark: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "spark.Base"`, func() {
		type invalidValue struct {
			Base  `spark:"foo/bar"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})

	assert.PanicsWithValue(t, `spark: expected to find a tag of the form 'spark:"name"' on "spark.Base"`, func() {
		type invalidValue struct {
			Base  `json:"-" spark:""`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidValue{})
	})
}

func TestDynamicAccess(t *testing.T) {
	key := &testMessage{}

	val, ok := stick.Get(key, "command")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(key, "Command")
	assert.True(t, ok)
	assert.Equal(t, "", val)

	ok = stick.Set(key, "command", "foo")
	assert.False(t, ok)
	assert.Equal(t, "", key.Command)

	ok = stick.Set(key, "Command", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", key.Command)
}
