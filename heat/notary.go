package heat

import (
	"fmt"
	"time"

	"github.com/256dpi/fire/coal"
)

// Notary is used to issue and verify tokens from keys.
type Notary struct {
	issuer string
	secret []byte
}

// NewNotary creates a new notary with the specified name and secret. It will
// panic if the name is missing or the specified secret is less that 16 bytes.
func NewNotary(name string, secret []byte) *Notary {
	// check name
	if name == "" {
		panic("heat: missing name")
	}

	// check secret
	if len(secret) < minSecretLen {
		panic("heat: secret too small")
	}

	return &Notary{
		secret: secret,
		issuer: name,
	}
}

// Issue will generate a token from the specified key.
func (n *Notary) Issue(key Key) (string, error) {
	// get meta
	meta := GetMeta(key)

	// get base
	base := key.GetBase()

	// ensure id
	if base.ID.IsZero() {
		base.ID = coal.New()
	}

	// ensure expiry
	if base.Expiry.IsZero() {
		base.Expiry = time.Now().Add(meta.Expiry)
	}

	// validate key
	err := key.Validate()
	if err != nil {
		return "", err
	}

	// get data
	var data coal.Map
	err = data.Marshal(key, coal.JSON)
	if err != nil {
		return "", err
	}

	// issue token
	token, err := Issue(n.secret, n.issuer, meta.Name, RawKey{
		ID:     base.ID.Hex(),
		Expiry: base.Expiry,
		Data:   data,
	})
	if err != nil {
		return "", err
	}

	return token, nil
}

// Verify will verify the specified token and fill the specified key.
func (n *Notary) Verify(key Key, token string) error {
	// get meta
	meta := GetMeta(key)

	// verify token
	rawKey, err := Verify(n.secret, n.issuer, meta.Name, token)
	if err != nil {
		return err
	}

	// check id
	kid, err := coal.FromHex(rawKey.ID)
	if err != nil {
		return fmt.Errorf("invalid token id")
	}

	// set id and expiry
	key.GetBase().ID = kid
	key.GetBase().Expiry = rawKey.Expiry

	// assign data
	err = rawKey.Data.Unmarshal(key, coal.JSON)
	if err != nil {
		return err
	}

	// validate key
	err = key.Validate()
	if err != nil {
		return err
	}

	return nil
}
