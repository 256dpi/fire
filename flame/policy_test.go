package flame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestPolicyParseAndGenerateToken(t *testing.T) {
	p := DefaultPolicy("")
	p.TokenData = func(c Client, ro ResourceOwner, t GenericToken) map[string]interface{} {
		return map[string]interface{}{
			"name": ro.(*User).Name,
		}
	}

	expiry := time.Now().Add(time.Hour)

	token := coal.Init(&Token{
		ExpiresAt: expiry,
	}).(*Token)
	sig, err := p.GenerateJWT(token, nil, &User{Name: "Hello"})
	assert.NoError(t, err)

	claims, _, err := p.ParseJWT(sig)
	assert.NoError(t, err)
	assert.Equal(t, token.ID().Hex(), claims.Id)
	assert.Equal(t, token.ID().Timestamp().Unix(), claims.IssuedAt)
	assert.Equal(t, expiry.Unix(), claims.ExpiresAt)
	assert.Equal(t, map[string]interface{}{
		"name": "Hello",
	}, claims.Data)
}
