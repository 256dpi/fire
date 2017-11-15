package flame

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestPolicyNewAccessToken(t *testing.T) {
	p := DefaultPolicy(testSecret)
	sig, err := p.NewAccessToken(bson.NewObjectId(), time.Now(), time.Now(), nil, nil)
	assert.NotEmpty(t, sig)
	assert.NoError(t, err)
}

func TestPolicyDataForAccessToken(t *testing.T) {
	id := bson.NewObjectId()
	tt := time.Now()

	p := DefaultPolicy(testSecret)
	p.DataForAccessToken = func(c Client, ro ResourceOwner) map[string]interface{} {
		return map[string]interface{}{
			"name": ro.(*User).Name,
		}
	}

	sig, err := p.NewAccessToken(id, tt, tt, nil, &User{Name: "Hello"})
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
