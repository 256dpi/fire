package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPolicyNewKeyAndSignature(t *testing.T) {
	p := DefaultPolicy(testSecret)
	key, sig, err := p.NewKeyAndSignature()
	assert.NotEmpty(t, key)
	assert.NotEmpty(t, sig)
	assert.NoError(t, err)
}
