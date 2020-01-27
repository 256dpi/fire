package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseID(t *testing.T) {
	post := &postModel{Base: NB()}
	assert.Equal(t, post.DocID, post.ID())
}

func TestGet(t *testing.T) {
	post := &postModel{}

	value, ok := Get(post, "TextBody")
	assert.Equal(t, "", value)
	assert.True(t, ok)

	post.TextBody = "hello"

	value, ok = Get(post, "TextBody")
	assert.Equal(t, "hello", value)
	assert.True(t, ok)

	value, ok = Get(post, "missing")
	assert.Nil(t, value)
	assert.False(t, ok)
}

func TestMustGet(t *testing.T) {
	post := &postModel{}

	assert.Equal(t, "", MustGet(post, "TextBody"))

	post.TextBody = "hello"

	assert.Equal(t, "hello", MustGet(post, "TextBody"))

	assert.PanicsWithValue(t, `coal: could not get field "missing" on "coal.postModel"`, func() {
		MustGet(post, "missing")
	})
}

func TestSet(t *testing.T) {
	post := &postModel{}

	ok := Set(post, "TextBody", "3")
	assert.True(t, ok)
	assert.Equal(t, "3", post.TextBody)

	ok = Set(post, "missing", "-")
	assert.False(t, ok)

	ok = Set(post, "TextBody", 1)
	assert.False(t, ok)
}

func TestMustSet(t *testing.T) {
	post := &postModel{}

	MustSet(post, "TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.PanicsWithValue(t, `coal: could not set "missing" on "coal.postModel"`, func() {
		MustSet(post, "missing", "-")
	})

	assert.PanicsWithValue(t, `coal: could not set "TextBody" on "coal.postModel"`, func() {
		MustSet(post, "TextBody", 1)
	})
}
