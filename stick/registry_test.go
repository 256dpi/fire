package stick

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type registryItem struct {
	Key1 string
	Key2 string
}

func (i *registryItem) Validate() error {
	return Validate(i, func(v *Validator) {
		v.Value("Key1", false, IsNotZero)
	})
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry[*registryItem](nil,
		func(item *registryItem) error {
			return item.Validate()
		},
		func(item *registryItem) string {
			return item.Key1
		},
		func(item *registryItem) string {
			return item.Key2
		},
	)

	v, ok := registry.Get(&registryItem{})
	assert.False(t, ok)
	assert.Nil(t, v)

	assert.PanicsWithValue(t, "stick: missing value", func() {
		registry.MustGet(&registryItem{})
	})

	item := &registryItem{
		Key1: "foo",
		Key2: "bar",
	}

	registry.Add(item)

	assert.PanicsWithValue(t, "stick: value already added", func() {
		registry.Add(item)
	})

	assert.PanicsWithValue(t, "stick: invalid value: Key1: zero", func() {
		registry.Add(&registryItem{})
	})

	assert.PanicsWithValue(t, "stick: missing key", func() {
		registry.Add(&registryItem{
			Key1: "baz",
		})
	})

	v, ok = registry.Get(&registryItem{
		Key1: "foo",
	})
	assert.True(t, ok)
	assert.Equal(t, item, v)

	assert.NotPanics(t, func() {
		registry.MustGet(&registryItem{
			Key2: "bar",
		})
	})

	assert.Equal(t, []*registryItem{item}, registry.All())

	v, ok = registry.Lookup(0, "foo")
	assert.True(t, ok)
	assert.Equal(t, item, v)

	v, ok = registry.Lookup(1, "foo")
	assert.False(t, ok)
	assert.NotEqual(t, item, v)

	v, ok = registry.Lookup(1, "bar")
	assert.True(t, ok)
	assert.Equal(t, item, v)
}
