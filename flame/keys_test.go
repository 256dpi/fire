package flame

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateKey(t *testing.T) {
	key1 := &testKey{}
	secret1, err := GenerateKey(key1, "tk_")
	assert.NoError(t, err)
	assert.Len(t, secret1, len("tk_")+43)
	assert.Equal(t, "tk_", secret1[:3])
	assert.Equal(t, hashKey(secret1), key1.Hash)
	assert.Len(t, key1.Hash, 32)
	assert.NotContains(t, string(key1.Hash), secret1)
	assert.Equal(t, secret1, key1.Secret)
	assert.Equal(t, secret1[:len("tk_")+FingerprintLength], key1.Fingerprint)
	assert.Nil(t, key1.Used)

	key2 := &testKey{}
	secret2, err := GenerateKey(key2, "tk_")
	assert.NoError(t, err)
	assert.NotEqual(t, secret1, secret2)
	assert.NotEqual(t, key1.Hash, key2.Hash)
	assert.NotEqual(t, key1.Fingerprint, key2.Fingerprint)
}

func TestGenerateKeyMinimal(t *testing.T) {
	key := &testMinimalKey{}
	secret, err := GenerateKey(key, "tk_")
	assert.NoError(t, err)
	assert.Equal(t, hashKey(secret), key.Hash)
}

func TestGenerateKeyMissingPrefix(t *testing.T) {
	key := &testKey{}
	secret, err := GenerateKey(key, "")
	assert.EqualError(t, err, "missing prefix")
	assert.Empty(t, secret)
	assert.Empty(t, key.Hash)
}

func TestPolicyGenerateKey(t *testing.T) {
	policy := keyPolicy()
	policy.KeyPrefix = "pk_"

	key := &testKey{}
	secret, err := policy.GenerateKey(key)
	assert.NoError(t, err)
	assert.Equal(t, "pk_", secret[:3])
	assert.Equal(t, hashKey(secret), key.Hash)
	assert.Equal(t, secret[:len("pk_")+FingerprintLength], key.Fingerprint)

	policy.KeyPrefix = ""
	secret, err = policy.GenerateKey(&testKey{})
	assert.EqualError(t, err, "missing prefix")
	assert.Empty(t, secret)
}

func TestKeyFields(t *testing.T) {
	assert.Equal(t, keyFields{
		hash:        "Hash",
		secret:      "Secret",
		fingerprint: "Fingerprint",
		used:        "Used",
	}, getKeyFields(&testKey{}))

	assert.Equal(t, keyFields{
		hash: "Hash",
	}, getKeyFields(&testMinimalKey{}))

	assert.Equal(t, keyFields{
		hash:        "Hash",
		secret:      "Secret",
		fingerprint: "Fingerprint",
		used:        "Used",
	}, getKeyFields(&Key{}))

	assert.Panics(t, func() {
		getKeyFields(&testMissingHashKey{})
	})

	assert.PanicsWithValue(t, `flame: expected "flame-key-hash" field to be a byte slice`, func() {
		getKeyFields(&testInvalidHashKey{})
	})

	assert.PanicsWithValue(t, `flame: expected "flame-key-secret" field to be a string`, func() {
		getKeyFields(&testInvalidSecretKey{})
	})

	assert.PanicsWithValue(t, `flame: expected "flame-key-fingerprint" field to be a string`, func() {
		getKeyFields(&testInvalidFingerprintKey{})
	})

	assert.PanicsWithValue(t, `flame: expected "flame-key-used" field to be a time pointer`, func() {
		getKeyFields(&testInvalidUsedKey{})
	})
}
