package kiln

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/stick"
)

type bsonProcess struct {
	Base `bson:"-" kiln:"bson"`
	Data string `bson:"data"`
}

func (p *bsonProcess) Validate() error {
	return nil
}

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&testProcess{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(testProcess{}),
		Name:   "test",
		Coding: stick.JSON,
		Accessor: &stick.Accessor{
			Name: "kiln.testProcess",
			Fields: map[string]*stick.Field{
				"Data": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	meta = GetMeta(&bsonProcess{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(bsonProcess{}),
		Name:   "bson",
		Coding: stick.BSON,
		Accessor: &stick.Accessor{
			Name: "kiln.bsonProcess",
			Fields: map[string]*stick.Field{
				"Data": {
					Index: 1,
					Type:  reflect.TypeOf(""),
				},
			},
		},
	}, meta)

	assert.PanicsWithValue(t, `kiln: expected first struct field to be an embedded "kiln.Base"`, func() {
		type invalidJob struct {
			Hello string
			Base
			stick.NoValidation
		}

		GetMeta(&invalidJob{})
	})

	assert.PanicsWithValue(t, `kiln: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "kiln.Base"`, func() {
		type invalidJob struct {
			Base  `kiln:"foo/bar"`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidJob{})
	})

	assert.PanicsWithValue(t, `kiln: expected to find a tag of the form 'kiln:"name"' on "kiln.Base"`, func() {
		type invalidJob struct {
			Base  `json:"-" kiln:""`
			Hello string
			stick.NoValidation
		}

		GetMeta(&invalidJob{})
	})
}

func TestMetaMake(t *testing.T) {
	proc := GetMeta(&testProcess{}).Make()
	assert.Equal(t, &testProcess{}, proc)
}

func TestDynamicAccess(t *testing.T) {
	proc := &testProcess{
		Data: "data",
	}

	val, ok := stick.Get(proc, "data")
	assert.False(t, ok)
	assert.Nil(t, val)

	val, ok = stick.Get(proc, "Data")
	assert.True(t, ok)
	assert.Equal(t, "data", val)

	ok = stick.Set(proc, "data", "foo")
	assert.False(t, ok)
	assert.Equal(t, "data", proc.Data)

	ok = stick.Set(proc, "Data", "foo")
	assert.True(t, ok)
	assert.Equal(t, "foo", proc.Data)
}
