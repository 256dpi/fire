package coal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestP(t *testing.T) {
	id := New()
	assert.Equal(t, &id, P(id))
}

func TestN(t *testing.T) {
	var id *ID
	assert.Equal(t, id, N())
	assert.NotEqual(t, nil, N())
}

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
