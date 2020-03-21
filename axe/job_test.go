package axe

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type invalidJob1 struct {
	Hello string
	Base
}

type invalidJob2 struct {
	Base  `axe:"foo/bar"`
	Hello string
}

type invalidJob3 struct {
	Base  `json:"-" axe:""`
	Hello string
}

type duplicateJob struct {
	Base  `json:"-" axe:"simple"`
	Hello string
}

func TestGetMeta(t *testing.T) {
	key := &simpleJob{
		Data: "cool",
	}

	meta := GetMeta(key)
	assert.Equal(t, &Meta{
		Type: reflect.TypeOf(simpleJob{}),
		Name: "simple",
	}, meta)

	data, err := json.Marshal(key)
	assert.NoError(t, err)
	assert.JSONEq(t, `{
		"data": "cool"
	}`, string(data))

	assert.PanicsWithValue(t, `axe: expected first struct field to be an embedded "axe.Base"`, func() {
		GetMeta(&invalidJob1{})
	})

	assert.PanicsWithValue(t, `axe: expected to find a tag of the form 'json:"-"' on "axe.Base"`, func() {
		GetMeta(&invalidJob2{})
	})

	assert.PanicsWithValue(t, `axe: expected to find a tag of the form 'axe:"name"' on "axe.Base"`, func() {
		GetMeta(&invalidJob3{})
	})

	assert.NotPanics(t, func() {
		GetMeta(&simpleJob{})
	})

	assert.PanicsWithValue(t, `axe: job name "simple" has already been registered by type "axe.simpleJob"`, func() {
		GetMeta(&duplicateJob{})
	})
}

func TestMetaMake(t *testing.T) {
	job := GetMeta(&simpleJob{}).Make()
	assert.Equal(t, &simpleJob{}, job)
}
