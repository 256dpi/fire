package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type validatableModel struct {
	Base `json:"-" bson:",inline" fire:"foo:foos"`
	Test string `valid:"required"`
}

func TestInit(t *testing.T) {
	m := Init(&Post{})
	assert.NotNil(t, m.Meta())
}

func TestInitSlice(t *testing.T) {
	m := InitSlice(&[]*Post{{}})
	assert.NotNil(t, m[0].Meta())
}

func TestBaseID(t *testing.T) {
	post := Init(&Post{}).(*Post)
	assert.Equal(t, post.DocID, post.ID())
}

func TestBaseGet(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, "", post1.Get("text_body"))
	assert.Equal(t, "", post1.Get("text-body"))
	assert.Equal(t, "", post1.Get("TextBody"))

	post2 := Init(&Post{TextBody: "hello"})
	assert.Equal(t, "hello", post2.Get("text_body"))
	assert.Equal(t, "hello", post2.Get("text-body"))
	assert.Equal(t, "hello", post2.Get("TextBody"))

	assert.Panics(t, func() {
		post1.Get("missing")
	})
}

func TestBaseSet(t *testing.T) {
	post := Init(&Post{}).(*Post)

	post.Set("text_body", "1")
	assert.Equal(t, "1", post.TextBody)

	post.Set("text-body", "2")
	assert.Equal(t, "2", post.TextBody)

	post.Set("TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.Panics(t, func() {
		post.Set("missing", "-")
	})

	assert.Panics(t, func() {
		post.Set("TextBody", 1)
	})
}

func TestBaseValidate(t *testing.T) {
	model := Init(&validatableModel{}).(*validatableModel)

	model.DocID = ""
	assert.Error(t, model.Validate(true))

	model.DocID = bson.NewObjectId()
	assert.Error(t, model.Validate(true))

	model.Test = "foo"
	assert.NoError(t, model.Validate(true))
}
