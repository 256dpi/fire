package heat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRand(t *testing.T) {
	bytes, err := Rand(32)
	assert.NoError(t, err)
	assert.Len(t, bytes, 32)

	assert.NotPanics(t, func() {
		MustRand(32)
	})
}
