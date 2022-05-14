package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsHex(t *testing.T) {
	assert.False(t, IsHex("foo"))
	assert.False(t, IsHex(""))
	assert.True(t, IsHex(New().Hex()))
}

func TestMustFromHex(t *testing.T) {
	assert.NotPanics(t, func() {
		MustFromHex(New().Hex())
	})

	assert.Panics(t, func() {
		MustFromHex("foo")
	})
}
