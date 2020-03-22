package stick

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type accessible struct {
	TextBody string
}

func (a *accessible) GetAccessor(interface{}) *Accessor {
	return &Accessor{
		Name: "stick.accessible",
		Fields: map[string]*Field{
			"TextBody": {
				Index: 0,
				Type:  reflect.TypeOf(""),
			},
		},
	}
}

func TestGet(t *testing.T) {
	post := &accessible{}

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
	post := &accessible{}

	assert.Equal(t, "", MustGet(post, "TextBody"))

	post.TextBody = "hello"

	assert.Equal(t, "hello", MustGet(post, "TextBody"))

	assert.PanicsWithValue(t, `stick: could not get field "missing" on "stick.accessible"`, func() {
		MustGet(post, "missing")
	})
}

func TestSet(t *testing.T) {
	post := &accessible{}

	ok := Set(post, "TextBody", "3")
	assert.True(t, ok)
	assert.Equal(t, "3", post.TextBody)

	ok = Set(post, "missing", "-")
	assert.False(t, ok)

	ok = Set(post, "TextBody", 1)
	assert.False(t, ok)
}

func TestMustSet(t *testing.T) {
	post := &accessible{}

	MustSet(post, "TextBody", "3")
	assert.Equal(t, "3", post.TextBody)

	assert.PanicsWithValue(t, `stick: could not set "missing" on "stick.accessible"`, func() {
		MustSet(post, "missing", "-")
	})

	assert.PanicsWithValue(t, `stick: could not set "TextBody" on "stick.accessible"`, func() {
		MustSet(post, "TextBody", 1)
	})
}
