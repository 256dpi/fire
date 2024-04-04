package roast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatrix(t *testing.T) {
	matrix := NewMatrix()

	matrix.Bool("foo")

	matrix.Values("bar", 1, 2)

	matrix.Generate("baz", []any{"a", "b", "c"}, func(key any, item Item) any {
		return "baz-" + key.(string)
	})

	assert.Equal(t, []Item{
		{"foo": true, "bar": 1},
		{"foo": true, "bar": 2},
		{"foo": false, "bar": 1},
		{"foo": false, "bar": 2},
	}, matrix.Items("bar"))

	assert.Equal(t, []Item{
		{"foo": true, "bar": 1, "baz": "baz-a"},
		{"foo": true, "bar": 1, "baz": "baz-b"},
		{"foo": true, "bar": 1, "baz": "baz-c"},
		{"foo": true, "bar": 2, "baz": "baz-a"},
		{"foo": true, "bar": 2, "baz": "baz-b"},
		{"foo": true, "bar": 2, "baz": "baz-c"},
		{"foo": false, "bar": 1, "baz": "baz-a"},
		{"foo": false, "bar": 1, "baz": "baz-b"},
		{"foo": false, "bar": 1, "baz": "baz-c"},
		{"foo": false, "bar": 2, "baz": "baz-a"},
		{"foo": false, "bar": 2, "baz": "baz-b"},
		{"foo": false, "bar": 2, "baz": "baz-c"},
	}, matrix.Items("baz"))

	assert.Equal(t, []Item{
		{"foo": true, "bar": 1, "baz": "baz-b"},
		{"foo": true, "bar": 2, "baz": "baz-b"},
		{"foo": false, "bar": 1, "baz": "baz-b"},
		{"foo": false, "bar": 2, "baz": "baz-b"},
	}, matrix.Items("baz", func(item Item) bool {
		return item["baz"] == "baz-b"
	}))

	assert.Equal(t, []Item{
		{"foo": true, "bar": 1, "baz": "baz-a"},
		{"foo": true, "bar": 1, "baz": "baz-c"},
		{"foo": true, "bar": 2, "baz": "baz-a"},
		{"foo": true, "bar": 2, "baz": "baz-c"},
		{"foo": false, "bar": 1, "baz": "baz-b"},
		{"foo": false, "bar": 2, "baz": "baz-b"},
	}, matrix.Items("baz", func(item Item) bool {
		return item["foo"].(bool) && item["baz"] != "baz-b"
	}, func(item Item) bool {
		return !item["foo"].(bool) && item["baz"] == "baz-b"
	}))

	for _, item := range matrix.Items("bar", func(item Item) bool {
		return item["bar"].(int) > 1
	}) {
		assert.True(t, item.Is("bar", 2))
		assert.Equal(t, 2, item["bar"])
	}

	assert.Panics(t, func() {
		matrix.Items("qux")
	})
}
