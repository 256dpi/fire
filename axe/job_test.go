package axe

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

type bsonJob struct {
	Base `bson:"-" axe:"bson"`
	Data string `bson:"data"`
}

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

func TestGetMeta(t *testing.T) {
	meta := GetMeta(&testJob{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(testJob{}),
		Name:   "test",
		Coding: coal.JSON,
	}, meta)

	meta = GetMeta(&bsonJob{})
	assert.Equal(t, &Meta{
		Type:   reflect.TypeOf(bsonJob{}),
		Name:   "bson",
		Coding: coal.BSON,
	}, meta)

	assert.PanicsWithValue(t, `axe: expected first struct field to be an embedded "axe.Base"`, func() {
		GetMeta(&invalidJob1{})
	})

	assert.PanicsWithValue(t, `axe: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "axe.Base"`, func() {
		GetMeta(&invalidJob2{})
	})

	assert.PanicsWithValue(t, `axe: expected to find a tag of the form 'axe:"name"' on "axe.Base"`, func() {
		GetMeta(&invalidJob3{})
	})
}

func TestMetaMake(t *testing.T) {
	job := GetMeta(&testJob{}).Make()
	assert.Equal(t, &testJob{}, job)
}
