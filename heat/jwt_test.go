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
	key2.Expiry = key2.Expiry.Local()
	assert.NoError(t, err)
	assert.Equal(t, key1, *key2)
}

func TestIssueErrors(t *testing.T) {
	token, err := Issue(nil, "", "", RawKey{})
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "secret too small", err.Error())

	token, err = Issue(MustRand(32), "", "", RawKey{})
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "missing issuer", err.Error())

	token, err = Issue(MustRand(32), "foo", "", RawKey{})
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "missing name", err.Error())

	token, err = Issue(MustRand(32), "foo", "bar", RawKey{})
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "missing id", err.Error())

	token, err = Issue(MustRand(32), "foo", "bar", RawKey{
		ID: "baz",
	})
	assert.Error(t, err)
	assert.Empty(t, token)
	assert.Equal(t, "missing expiry", err.Error())

	token, err = Issue(MustRand(32), "foo", "bar", RawKey{
		ID: "baz",
		Expiry: time.Now().Add(time.Hour),
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
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

func TestVerifyErrors(t *testing.T) {
	key, err := Verify(nil, "", "", "")
	assert.Error(t, err)
	assert.Nil(t, key)
	assert.Equal(t, "secret too small", err.Error())

	key, err = Verify(MustRand(32), "", "", "")
	assert.Error(t, err)
	assert.Nil(t, key)
	assert.Equal(t, "missing issuer", err.Error())

	key, err = Verify(MustRand(32), "foo", "", "")
	assert.Error(t, err)
	assert.Nil(t, key)
	assert.Equal(t, "missing name", err.Error())

	key, err = Verify(MustRand(32), "foo", "bar", "")
	assert.Error(t, err)
	assert.Nil(t, key)
	assert.Equal(t, "invalid token", err.Error())
}
