package stick

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnique(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, Unique([]string{"a", "b", "c"}))
	assert.Equal(t, []string{"a", "b"}, Unique([]string{"a", "a", "b"}))
	assert.Equal(t, []string{"a", "b"}, Unique([]string{"a", "b", "b"}))
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

func TestUnion(t *testing.T) {
	assert.Equal(t, []string{"b", "a", "c", "d"}, Union([]string{"b", "a", "c"}, []string{"d", "a", "b"}))
}

func TestIntersect(t *testing.T) {
	assert.Equal(t, []string{"b"}, Intersect([]string{"a", "b"}, []string{"b", "c"}))
}
