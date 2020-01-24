package flame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

func TestPolicyParseAndGenerateToken(t *testing.T) {
	p := DefaultPolicy("")
	p.TokenData = func(c Client, ro ResourceOwner, t GenericToken) map[string]interface{} {
		return map[string]interface{}{
			"name": ro.(*User).Name,
		}
	}

	expiry := time.Now().Add(time.Hour).Round(time.Second)
	token := coal.Init(&Token{ExpiresAt: expiry}).(*Token)

	sig, err := p.GenerateJWT(token, nil, &User{Name: "Hello"})
	assert.NoError(t, err)

	key, err := p.ParseJWT(sig)
	assert.NoError(t, err)
	assert.Equal(t, token.ID().Hex(), key.ID)
	assert.Equal(t, expiry, key.Expiry)
	assert.Equal(t, heat.Data{
		"name": "Hello",
	}, key.Data)
}
