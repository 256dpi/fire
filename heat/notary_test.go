package heat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire/coal"
)

func TestNotaryIssueAndVerify(t *testing.T) {
	notary := NewNotary("test", testSecret)

	key1 := testKey{
		Base: Base{
			ID:      coal.New(),
			Issued:  time.Now().Add(-time.Second).Round(time.Second),
			Expires: time.Now().Add(time.Hour).Round(time.Second),
		},
		User: "user1234",
		Role: "admin",
	}

	token, err := notary.Issue(&key1)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	var key2 testKey
	err = notary.Verify(&key2, token)
	assert.NoError(t, err)
	key2.Issued = key2.Issued.Local()
	key2.Expires = key2.Expires.Local()
	assert.Equal(t, key1, key2)
}

func TestNotaryIssueDefaults(t *testing.T) {
	notary := NewNotary("test", testSecret)

	token, err := notary.Issue(&testKey{
		User: "user",
		Role: "role",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	var key testKey
	err = notary.Verify(&key, token)
	assert.NoError(t, err)
	assert.False(t, key.ID.IsZero())
	assert.True(t, time.Until(key.Expires) > time.Hour-time.Minute)
	assert.Equal(t, "user", key.User)
	assert.Equal(t, "role", key.Role)
}

func TestNotaryIssueValidation(t *testing.T) {
	notary := NewNotary("test", testSecret)

	token, err := notary.Issue(&testKey{
		User: "user",
	})
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "missing role", err.Error())
}

func TestNotaryVerifyErrors(t *testing.T) {
	notary := NewNotary("test", testSecret)

	var key testKey
	err := notary.Verify(&key, makeToken(jwtClaims{
		Issuer:   "test",
		Audience: "test",
		ID:       "invalid",
		Issued:   time.Now().Unix(),
		Expires:  time.Now().Add(time.Hour).Unix(),
		Data:     nil,
	}))
	assert.Error(t, err)
	assert.Equal(t, "invalid token id", err.Error())

	err = notary.Verify(&key, makeToken(jwtClaims{
		Issuer:   "test",
		Audience: "test",
		ID:       coal.New().Hex(),
		Issued:   time.Now().Unix(),
		Expires:  time.Now().Add(time.Hour).Unix(),
		Data:     nil,
	}))
	assert.Error(t, err)
	assert.Equal(t, "missing user", err.Error())
}

func TestNewNotaryPanics(t *testing.T) {
	assert.PanicsWithValue(t, `heat: missing name`, func() {
		NewNotary("", nil)
	})

	assert.PanicsWithValue(t, `heat: secret too small`, func() {
		NewNotary("foo", nil)
	})

	assert.NotPanics(t, func() {
		NewNotary("foo", testSecret)
	})
}
