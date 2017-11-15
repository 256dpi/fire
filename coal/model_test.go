package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestInit(t *testing.T) {
	m := Init(&postModel{})
	assert.NotNil(t, m.Meta())
}

func TestInitSlice(t *testing.T) {
	m := InitSlice(&[]*postModel{{}})
	assert.NotNil(t, m[0].Meta())
}

func TestC(t *testing.T) {
	assert.Equal(t, "posts", C(&postModel{}))
}

func TestF(t *testing.T) {
	assert.Equal(t, "text_body", F(&postModel{}, "TextBody"))
}

func TestA(t *testing.T) {
	assert.Equal(t, "text-body", A(&postModel{}, "TextBody"))
}

func TestValidate(t *testing.T) {
	post := Init(&postModel{}).(*postModel)

	post.DocID = ""
	assert.Error(t, Validate(post))

	post.DocID = bson.NewObjectId()
	assert.Error(t, Validate(post))

	post.Title = "foo"
	assert.NoError(t, Validate(post))
}

func TestValidateTimestamps(t *testing.T) {
	note := Init(&noteModel{}).(*noteModel)
	err := note.Validate()
	assert.NoError(t, err)
	assert.NotEmpty(t, note.CreatedAt)
	assert.NotEmpty(t, note.UpdatedAt)
}

func TestBaseID(t *testing.T) {
	post := Init(&postModel{}).(*postModel)
	assert.Equal(t, post.DocID, post.ID())
}

func TestBaseGet(t *testing.T) {
	post1 := Init(&postModel{})
	assert.Equal(t, "", post1.MustGet("text_body"))
	assert.Equal(t, "", post1.MustGet("text-body"))
	assert.Equal(t, "", post1.MustGet("TextBody"))

	post2 := Init(&postModel{TextBody: "hello"})
	assert.Equal(t, "hello", post2.MustGet("text_body"))
	assert.Equal(t, "hello", post2.MustGet("text-body"))
	assert.Equal(t, "hello", post2.MustGet("TextBody"))

	assert.Panics(t, func() {
		post1.MustGet("missing")
	})
}

func TestBaseSet(t *testing.T) {
	post := Init(&postModel{}).(*postModel)

	post.MustSet("text_body", "1")
	assert.Equal(t, "1", post.TextBody)

	post.MustSet("text-body", "2")
	assert.Equal(t, "2", post.TextBody)

	post.MustSet("TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.Panics(t, func() {
		post.MustSet("missing", "-")
	})

	assert.Panics(t, func() {
		post.MustSet("TextBody", 1)
	})
}
