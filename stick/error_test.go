package stick

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

func TestSafeError(t *testing.T) {
	err1 := errors.New("foo")
	assert.False(t, IsSafe(err1))
	assert.Equal(t, "foo", err1.Error())
	assert.Nil(t, errors.Unwrap(err1))

	err2 := Safe(err1)
	assert.True(t, IsSafe(err2))
	assert.Equal(t, "foo", err2.Error())
	assert.Equal(t, err1, errors.Unwrap(err2))
}
