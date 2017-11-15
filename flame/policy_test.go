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
