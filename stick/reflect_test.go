package stick

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnwrap(t *testing.T) {
	str := ""
	ptr := &str
	assert.Equal(t, str, Unwrap(&ptr))
}

func TestIsNil(t *testing.T) {
	assert.True(t, IsNil(nil))
	assert.True(t, IsNil((*time.Time)(nil)))
	assert.False(t, IsNil(1))
	assert.False(t, IsNil(&time.Time{}))
}
