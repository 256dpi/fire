package stick

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type accessible struct {
	String    string
	OptString *string
	BasicAccess
}

func TestBuildAccessor(t *testing.T) {
	acc := &accessible{}

	assert.Equal(t, &Accessor{
		Name: "stick.accessible",
		Fields: map[string]*Field{
			"String": {
				Index: 0,
				Type:  reflect.TypeOf(""),
			},
			"OptString": {
				Index: 1,
				Type:  reflect.PtrTo(reflect.TypeOf("")),
			},
		},
	}, acc.GetAccessor(acc))
}

func TestGet(t *testing.T) {
	acc := &accessible{}

	value, ok := Get(acc, "String")
	assert.Equal(t, "", value)
	assert.True(t, ok)

	acc.String = "hello"

	value, ok = Get(acc, "String")
	assert.Equal(t, "hello", value)
	assert.True(t, ok)

	value, ok = Get(acc, "missing")
	assert.Nil(t, value)
	assert.False(t, ok)
}

func TestMustGet(t *testing.T) {
	acc := &accessible{}
	assert.Equal(t, "", MustGet(acc, "String"))

	acc.String = "hello"
	assert.Equal(t, "hello", MustGet(acc, "String"))

	assert.PanicsWithValue(t, `stick: could not get field "missing" on "stick.accessible"`, func() {
		MustGet(acc, "missing")
	})
}

func TestSet(t *testing.T) {
	acc := &accessible{}

	ok := Set(acc, "String", "3")
	assert.True(t, ok)
	assert.Equal(t, "3", acc.String)

	ok = Set(acc, "missing", "-")
	assert.False(t, ok)

	ok = Set(acc, "String", 1)
	assert.False(t, ok)
}

func TestSetNil(t *testing.T) {
	acc := &accessible{}

	ok := Set(acc, "OptString", nil)
	assert.True(t, ok)

	ok = Set(acc, "OptString", (*string)(nil))
	assert.True(t, ok)
}

func TestMustSet(t *testing.T) {
	acc := &accessible{}

	MustSet(acc, "String", "3")
	assert.Equal(t, "3", acc.String)

	assert.PanicsWithValue(t, `stick: could not set "missing" on "stick.accessible"`, func() {
		MustSet(acc, "missing", "-")
	})

	assert.PanicsWithValue(t, `stick: could not set "String" on "stick.accessible"`, func() {
		MustSet(acc, "String", 1)
	})
}

func BenchmarkBuildAccessor(b *testing.B) {
	acc := &accessible{}

	for i := 0; i < b.N; i++ {
		BuildAccessor(acc)
	}
}
