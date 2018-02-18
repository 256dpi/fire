package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	m := Init(&postModel{})
	assert.NotNil(t, m.Meta())
}

func TestInitSlice(t *testing.T) {
	m := InitSlice(&[]*postModel{{}})
	assert.NotNil(t, m[0].Meta())
}

func TestBaseID(t *testing.T) {
	post := Init(&postModel{}).(*postModel)
	assert.Equal(t, post.DocID, post.ID())
}

func TestBaseGet(t *testing.T) {
	post1 := Init(&postModel{})
	assert.Equal(t, "", post1.MustGet("TextBody"))

	post2 := Init(&postModel{TextBody: "hello"})
	assert.Equal(t, "hello", post2.MustGet("TextBody"))

	assert.PanicsWithValue(t, `coal: field "missing" not found on "coal.postModel"`, func() {
		post1.MustGet("missing")
	})
}

func TestBaseSet(t *testing.T) {
	post := Init(&postModel{}).(*postModel)

	post.MustSet("TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.PanicsWithValue(t, `coal: field "missing" not found on "coal.postModel"`, func() {
		post.MustSet("missing", "-")
	})

	assert.PanicsWithValue(t, `reflect.Set: value of type int is not assignable to type string`, func() {
		post.MustSet("TextBody", 1)
	})
}
