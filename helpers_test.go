package fire

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestE(t *testing.T) {
	err := E("foo")
	assert.True(t, IsSafe(err))
	assert.Equal(t, "foo", err.Error())
}

func TestSafe(t *testing.T) {
	err := Safe(errors.New("foo"))
	assert.True(t, IsSafe(err))
	assert.Equal(t, "foo", err.Error())

	err = errors.New("foo")
	assert.False(t, IsSafe(err))
	assert.Equal(t, "foo", err.Error())
}

func TestContains(t *testing.T) {
	assert.True(t, Contains([]string{"a", "b", "c"}, "a"))
	assert.True(t, Contains([]string{"a", "b", "c"}, "b"))
	assert.True(t, Contains([]string{"a", "b", "c"}, "c"))
	assert.False(t, Contains([]string{"a", "b", "c"}, "d"))
}

func TestIncludes(t *testing.T) {
	assert.True(t, Includes([]string{"a", "b", "c"}, []string{"a"}))
	assert.True(t, Includes([]string{"a", "b", "c"}, []string{"a", "b"}))
	assert.True(t, Includes([]string{"a", "b", "c"}, []string{"a", "b", "c"}))
	assert.False(t, Includes([]string{"a", "b", "c"}, []string{"a", "b", "c", "d"}))
	assert.False(t, Includes([]string{"a", "b", "c"}, []string{"d"}))
}

func TestIntersect(t *testing.T) {
	assert.Equal(t, []string{"b"}, Intersect([]string{"a", "b"}, []string{"b", "c"}))
}
