package glut

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type invalidValue1 struct {
	Hello string
	Base
}

type invalidValue2 struct {
	Base  `glut:"foo/bar"`
	Hello string
}

type invalidValue3 struct {
	Base  `json:"-" glut:""`
	Hello string
}

type invalidValue4 struct {
	Base  `json:"-" glut:"foo,bar"`
	Hello string
}

func TestGetMeta(t *testing.T) {
	key := &testValue{
		Data: "cool",
	}

	meta := GetMeta(key)
	assert.Equal(t, &Meta{
		Type: reflect.TypeOf(&testValue{}),
		Key:  "test",
		TTL:  0,
	}, meta)

	data, err := json.Marshal(key)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"data": "cool"
	}`, string(data))

	assert.PanicsWithValue(t, `glut: expected first struct field to be an embedded "glut.Base"`, func() {
		GetMeta(&invalidValue1{})
	})

	assert.PanicsWithValue(t, `glut: expected to find a tag of the form 'json:"-"' on "glut.Base"`, func() {
		GetMeta(&invalidValue2{})
	})

	assert.PanicsWithValue(t, `glut: expected to find a tag of the form 'glut:"key,ttl"' on "glut.Base"`, func() {
		GetMeta(&invalidValue3{})
	})

	assert.PanicsWithValue(t, `glut: invalid duration as time to live on "glut.Base"`, func() {
		GetMeta(&invalidValue4{})
	})
}
