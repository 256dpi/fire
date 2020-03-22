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
