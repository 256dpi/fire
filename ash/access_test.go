package ash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessTable(t *testing.T) {
	table := AccessTable{
		"Foo": Find | Create | Delete,
		"Bar": Read,
	}

	assert.Equal(t, []string{"Foo", "Bar"}, table.Collect(Read))
	assert.Equal(t, []string{"Foo"}, table.Collect(Write))
}

func TestAccessMatrix(t *testing.T) {
	m := AccessMatrix{
		"Foo": {"FCD", "*"},
		"Bar": {"R  ", "W"},
	}

	assert.Equal(t, AccessTable{
		"Foo": Find | Create | Delete,
		"Bar": Read,
	}, m.Compile(0))

	assert.Equal(t, AccessTable{
		"Foo": Full,
		"Bar": Write,
	}, m.Compile(1))

	assert.Equal(t, AccessTable{
		"Foo": Full,
		"Bar": Full,
	}, m.Compile(0, 1))
}

func TestNamedAccessMatrix(t *testing.T) {
	m := NamedAccessMatrix{
		Columns: []string{"foo", "bar"},
		Matrix: AccessMatrix{
			"Foo": {"FCD", "*"},
			"Bar": {"R  ", "W"},
		},
	}

	assert.Equal(t, AccessTable{
		"Foo": Find | Create | Delete,
		"Bar": Read,
	}, m.Compile("foo"))

	assert.Equal(t, AccessTable{
		"Foo": Full,
		"Bar": Write,
	}, m.Compile("bar"))

	assert.Equal(t, AccessTable{
		"Foo": Full,
		"Bar": Full,
	}, m.Compile("foo", "bar"))

	assert.PanicsWithValue(t, "ash: column not found", func() {
		m.Compile("baz")
	})
}
