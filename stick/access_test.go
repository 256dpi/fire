package stick

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccess(t *testing.T) {
	var foo struct {
		Foo string
	}

	acc1 := GetAccessor(foo)
	acc2 := GetAccessor(&foo)
	assert.True(t, acc1 == acc2)
	assert.Equal(t, &Accessor{
		Name: "struct { Foo string }",
		Fields: map[string]*Field{
			"Foo": {
				Index: 0,
				Type:  reflect.TypeOf(""),
			},
		},
	}, acc1)

	MustSet(&foo, "Foo", "bar")
	ret := MustGet(foo, "Foo")
	assert.Equal(t, "bar", ret)
}

type accessible struct {
	String    string
	OptString *string
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
	}, GetAccessor(acc))
}

func TestGet(t *testing.T) {
	assert.PanicsWithValue(t, "stick: nil value", func() {
		Get((*accessible)(nil), "String")
	})

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

func TestGetRaw(t *testing.T) {
	assert.PanicsWithValue(t, "stick: nil value", func() {
		GetRaw((*accessible)(nil), "String")
	})

	acc := &accessible{}

	value, ok := GetRaw(acc, "String")
	assert.Equal(t, "", value.String())
	assert.True(t, ok)

	acc.String = "hello"

	value, ok = GetRaw(acc, "String")
	assert.Equal(t, "hello", value.String())
	assert.True(t, ok)

	value, ok = GetRaw(acc, "missing")
	assert.Zero(t, value)
	assert.False(t, ok)
}

func TestMustGetRaw(t *testing.T) {
	acc := &accessible{}
	assert.Equal(t, "", MustGetRaw(acc, "String").String())

	acc.String = "hello"
	assert.Equal(t, "hello", MustGetRaw(acc, "String").String())

	assert.PanicsWithValue(t, `stick: could not get field "missing" on "stick.accessible"`, func() {
		MustGet(acc, "missing")
	})
}

func TestSet(t *testing.T) {
	assert.PanicsWithValue(t, "stick: nil value", func() {
		Set((*accessible)(nil), "String", "foo")
	})

	assert.PanicsWithValue(t, "stick: not addressable", func() {
		Set(accessible{}, "String", "foo")
	})

	acc := &accessible{}

	ok := Set(acc, "String", "3")
	assert.True(t, ok)
	assert.Equal(t, "3", acc.String)

	ok = Set(acc, "missing", "-")
	assert.False(t, ok)

	ok = Set(acc, "String", 1)
	assert.False(t, ok)

	ok = Set(acc, "OptString", nil)
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

func BenchmarkMustGet(b *testing.B) {
	acc := &accessible{}

	for i := 0; i < b.N; i++ {
		MustGet(acc, "String")
	}
}

func BenchmarkMustSet(b *testing.B) {
	acc := &accessible{}

	for i := 0; i < b.N; i++ {
		MustSet(acc, "String", "foo")
	}
}
