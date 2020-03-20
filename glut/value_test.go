package glut

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type simpleValue struct {
	Base `json:"-" glut:"value/simple,0"`

	Data string `json:"data"`
}

type ttlValue struct {
	Base `json:"-" glut:"value/ttl,5m"`

	Data string `json:"data"`
}

func TestMeta(t *testing.T) {
	key := &simpleValue{
		Data: "cool",
	}

	meta := Meta(key)
	assert.Equal(t, ValueMeta{
		Key: "value/simple",
	}, meta)

	data, err := json.Marshal(key)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"data": "cool"
	}`, string(data))
}

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
	Base  `json:"-" glut:"invalidValue4,foo"`
	Hello string
}

type duplicateValue struct {
	Base  `json:"-" glut:"value/simple,0"`
	Hello string
}

func TestMetaPanics(t *testing.T) {
	assert.PanicsWithValue(t, `glut: expected first struct field to be an embedded "glut.Base"`, func() {
		Meta(&invalidValue1{})
	})

	assert.PanicsWithValue(t, `glut: expected to find a tag of the form 'json:"-"' on "glut.Base"`, func() {
		Meta(&invalidValue2{})
	})

	assert.PanicsWithValue(t, `glut: expected to find a tag of the form 'glut:"key,ttl"' on "glut.Base"`, func() {
		Meta(&invalidValue3{})
	})

	assert.PanicsWithValue(t, `glut: invalid duration as TTL on "glut.Base"`, func() {
		Meta(&invalidValue4{})
	})

	assert.NotPanics(t, func() {
		Meta(&simpleValue{})
	})

	assert.PanicsWithValue(t, `glut: value key "value/simple" has already been registered by type "*glut.simpleValue"`, func() {
		Meta(&duplicateValue{})
	})
}
