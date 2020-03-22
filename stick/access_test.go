package stick

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type accessible struct {
	String string
}

func (a *accessible) GetAccessor(v interface{}) *Accessor {
	return BuildAccessor(v)
}

func TestBuildAccessor(t *testing.T) {
	accessor := BuildAccessor(accessible{})
	assert.Equal(t, &Accessor{
		Name: "stick.accessible",
		Fields: map[string]*Field{
			"String": {
				Index: 0,
				Type:  reflect.TypeOf(""),
			},
		},
	}, accessor)
}

func TestGet(t *testing.T) {
	post := &accessible{}

	value, ok := Get(post, "String")
	assert.Equal(t, "", value)
	assert.True(t, ok)

	post.String = "hello"

	value, ok = Get(post, "String")
	assert.Equal(t, "hello", value)
	assert.True(t, ok)

	value, ok = Get(post, "missing")
	assert.Nil(t, value)
	assert.False(t, ok)
}

func TestMustGet(t *testing.T) {
	post := &accessible{}
	assert.Equal(t, "", MustGet(post, "String"))

	post.String = "hello"
	assert.Equal(t, "hello", MustGet(post, "String"))

	assert.PanicsWithValue(t, `stick: could not get field "missing" on "stick.accessible"`, func() {
		MustGet(post, "missing")
	})
}

func TestSet(t *testing.T) {
	post := &accessible{}

	ok := Set(post, "String", "3")
	assert.True(t, ok)
	assert.Equal(t, "3", post.String)

	ok = Set(post, "missing", "-")
	assert.False(t, ok)

	ok = Set(post, "String", 1)
	assert.False(t, ok)
}

func TestMustSet(t *testing.T) {
	post := &accessible{}

	MustSet(post, "String", "3")
	assert.Equal(t, "3", post.String)

	assert.PanicsWithValue(t, `stick: could not set "missing" on "stick.accessible"`, func() {
		MustSet(post, "missing", "-")
	})

	assert.PanicsWithValue(t, `stick: could not set "String" on "stick.accessible"`, func() {
		MustSet(post, "String", 1)
	})
}
