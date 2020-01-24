package heat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIssueAndVerify(t *testing.T) {
	secret := MustRand(32)

	key1 := RawKey{
		ID:     "id",
		Expiry: time.Now().Add(time.Hour).Round(time.Second),
		Data: Data{
			"user": "user",
			"role": "role",
		},
	}

	token, err := Issue(secret, "issuer", "name", key1)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	key2, err := Verify(secret, "issuer", "name", token)
	assert.NoError(t, err)
	assert.Equal(t, key1, *key2)
}

func TestVerifyExpired(t *testing.T) {
	secret := MustRand(32)

	token, err := Issue(secret, "issuer", "name", RawKey{
		ID:     "id",
		Expiry: time.Now().Add(-time.Hour).Round(time.Second),
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	key2, err := Verify(secret, "issuer", "name", token)
	assert.Error(t, err)
	assert.Nil(t, key2)
	assert.Equal(t, ErrExpiredToken, err)
}

func TestVerifyInvalid(t *testing.T) {
	secret1 := MustRand(32)
	secret2 := MustRand(32)

	token, err := Issue(secret1, "issuer", "name", RawKey{
		ID:     "id",
		Expiry: time.Now().Add(time.Hour).Round(time.Second),
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	key2, err := Verify(secret2, "issuer", "name", token)
	assert.Error(t, err)
	assert.Nil(t, key2)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestVerifyExpiredAndInvalid(t *testing.T) {
	secret1 := MustRand(32)
	secret2 := MustRand(32)

	token, err := Issue(secret1, "issuer", "name", RawKey{
		ID:     "id",
		Expiry: time.Now().Add(-time.Hour).Round(time.Second),
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	key2, err := Verify(secret2, "issuer", "name", token)
	assert.Error(t, err)
	assert.Nil(t, key2)
	assert.Equal(t, ErrInvalidToken, err)
}
