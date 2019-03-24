package flame

import (
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
)

func TestPolicyParseAndGenerateToken(t *testing.T) {
	id := bson.NewObjectId()
	tt := time.Now()

	p := DefaultPolicy("")
	p.TokenData = func(c Client, ro ResourceOwner, t GenericToken) map[string]interface{} {
		return map[string]interface{}{
			"name": ro.(*User).Name,
		}
	}

	sig, err := p.GenerateToken(id, tt, tt, nil, &User{Name: "Hello"}, nil)
	assert.NoError(t, err)

	claims, _, err := p.ParseToken(sig)
	assert.NoError(t, err)
	assert.Equal(t, id.Hex(), claims.Id)
	assert.Equal(t, tt.Unix(), claims.IssuedAt)
	assert.Equal(t, tt.Unix(), claims.ExpiresAt)
	assert.Equal(t, map[string]interface{}{
		"name": "Hello",
	}, claims.Data)
}
