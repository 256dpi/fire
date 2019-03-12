package spark

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPolicyParseAndGenerateToken(t *testing.T) {
	tt := time.Now()

	p := DefaultPolicy("")

	sig, err := p.GenerateToken("items", "1", tt, tt, map[string]interface{}{
		"name": "Hello",
	})
	assert.NoError(t, err)

	claims, _, err := p.ParseToken(sig)
	assert.NoError(t, err)
	assert.Equal(t, "items", claims.Subject)
	assert.Equal(t, "1", claims.Id)
	assert.Equal(t, tt.Unix(), claims.IssuedAt)
	assert.Equal(t, tt.Unix(), claims.ExpiresAt)
	assert.Equal(t, map[string]interface{}{
		"name": "Hello",
	}, claims.Data)
}
