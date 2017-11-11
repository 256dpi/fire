package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestPolicyNewKeyAndSignature(t *testing.T) {
	p := DefaultPolicy(testSecret)
	sig, err := p.NewAccessToken(bson.NewObjectId(), time.Now(), time.Now(), nil)
	assert.NotEmpty(t, sig)
	assert.NoError(t, err)
}
