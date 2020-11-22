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
