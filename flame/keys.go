package flame

import (
	"crypto/sha256"
	"encoding/base64"
	"reflect"
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// FingerprintLength is the number of secret characters following the prefix
// that make up the fingerprint of a key.
const FingerprintLength = 8

// KeyUseInterval is the interval at which the use of a key is recorded.
const KeyUseInterval = time.Minute

var bytesType = reflect.TypeOf([]byte{})
var stringType = reflect.TypeOf("")
var optTimeType = reflect.TypeOf(&time.Time{})

type keyFields struct {
	hash        string
	secret      string
	fingerprint string
	used        string
}

// getKeyFields will look up the flagged fields of a key model as described by
// GenericKey. It will panic if the hash field is missing or a field has the
// wrong type.
func getKeyFields(model GenericKey) keyFields {
	// lookup fields
	fields := keyFields{
		hash:        coal.L(model, "flame-key-hash", true),
		secret:      coal.L(model, "flame-key-secret", false),
		fingerprint: coal.L(model, "flame-key-fingerprint", false),
		used:        coal.L(model, "flame-key-used", false),
	}

	// check field types
	meta := coal.GetMeta(model).Fields
	if meta[fields.hash].Type != bytesType {
		panic(`flame: expected "flame-key-hash" field to be a byte slice`)
	}
	if fields.secret != "" && meta[fields.secret].Type != stringType {
		panic(`flame: expected "flame-key-secret" field to be a string`)
	}
	if fields.fingerprint != "" && meta[fields.fingerprint].Type != stringType {
		panic(`flame: expected "flame-key-fingerprint" field to be a string`)
	}
	if fields.used != "" && meta[fields.used].Type != optTimeType {
		panic(`flame: expected "flame-key-used" field to be a time pointer`)
	}

	return fields
}

// GenerateKey will generate a new secret for the specified key, set its hash,
// fingerprint and secret field and return the secret. The secret consists of
// the prefix and 32 random bytes. It is only stored as a hash and must be
// handed out by the caller, who also stores the key.
//
// Keys are stored rather than signed like access tokens: a secret is verified
// by looking up its hash, and a key is revoked by deleting it.
func GenerateKey(key GenericKey, prefix string) (string, error) {
	// check prefix
	if prefix == "" {
		return "", xo.F("missing prefix")
	}

	// get fields
	fields := getKeyFields(key)

	// generate secret
	bytes, err := heat.Rand(32)
	if err != nil {
		return "", err
	}

	// encode secret
	secret := prefix + base64.RawURLEncoding.EncodeToString(bytes)

	// set hash
	stick.MustSet(key, fields.hash, hashKey(secret))

	// set secret
	if fields.secret != "" {
		stick.MustSet(key, fields.secret, secret)
	}

	// set fingerprint
	if fields.fingerprint != "" {
		stick.MustSet(key, fields.fingerprint, secret[:len(prefix)+FingerprintLength])
	}

	return secret, nil
}

// hashKey returns the hash a secret is stored and looked up by. A plain hash
// suffices as secrets are random and long, which also allows looking them up.
func hashKey(secret string) []byte {
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}
