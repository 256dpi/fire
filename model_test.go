package fire

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type postWithTimestamps struct {
	Base      `json:"-" bson:",inline" fire:"posts"`
	Title     string    `json:"title" bson:"title" valid:"required"`
	CreatedAt time.Time `json:"created-at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated-at" bson:"updated_at"`
}

func (p *postWithTimestamps) Validate() error {
	ValidateTimestamps(p, "CreatedAt", "UpdatedAt")
	return nil
}

func TestInit(t *testing.T) {
	m := Init(&Post{})
	assert.NotNil(t, m.Meta())
}

func TestInitSlice(t *testing.T) {
	m := InitSlice(&[]*Post{{}})
	assert.NotNil(t, m[0].Meta())
}

func TestC(t *testing.T) {
	assert.Equal(t, "posts", C(&Post{}))
}

func TestF(t *testing.T) {
	assert.Equal(t, "text_body", F(&Post{}, "TextBody"))
}

func TestA(t *testing.T) {
	assert.Equal(t, "text-body", A(&Post{}, "TextBody"))
}

func TestValidate(t *testing.T) {
	post := Init(&Post{}).(*Post)

	post.DocID = ""
	assert.Error(t, Validate(post))

	post.DocID = bson.NewObjectId()
	assert.Error(t, Validate(post))

	post.Title = "foo"
	assert.NoError(t, Validate(post))
}

func TestValidateTimestamps(t *testing.T) {
	post := Init(&postWithTimestamps{}).(*postWithTimestamps)
	err := post.Validate()
	assert.NoError(t, err)
	assert.NotEmpty(t, post.CreatedAt)
	assert.NotEmpty(t, post.UpdatedAt)
}

func TestBaseID(t *testing.T) {
	post := Init(&Post{}).(*Post)
	assert.Equal(t, post.DocID, post.ID())
}

func TestBaseGet(t *testing.T) {
	post1 := Init(&Post{})
	assert.Equal(t, "", post1.MustGet("text_body"))
	assert.Equal(t, "", post1.MustGet("text-body"))
	assert.Equal(t, "", post1.MustGet("TextBody"))

	post2 := Init(&Post{TextBody: "hello"})
	assert.Equal(t, "hello", post2.MustGet("text_body"))
	assert.Equal(t, "hello", post2.MustGet("text-body"))
	assert.Equal(t, "hello", post2.MustGet("TextBody"))

	assert.Panics(t, func() {
		post1.MustGet("missing")
	})
}

func TestBaseSet(t *testing.T) {
	post := Init(&Post{}).(*Post)

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
