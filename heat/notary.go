package heat

import (
	"fmt"
	"time"

	"github.com/256dpi/jsonapi/v2"

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
	if len(secret) < 16 {
		panic("heat: secret too small")
	}

	return &Notary{
		secret: secret,
		issuer: name,
	}
}

// Issue will generate a token from the specified key.
func (n *Notary) Issue(key Key) (string, error) {
	// get key meta
	meta := Meta(key)

	// get base
	base := key.base()

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
	data, err := jsonapi.StructToMap(key, nil)
	if err != nil {
		return "", err
	}

	// issue token
	token, err := Issue(n.secret, n.issuer, meta.Name, RawKey{
		ID:     base.ID.Hex(),
		Expiry: base.Expiry,
		Data:   Data(data),
	})
	if err != nil {
		return "", err
	}

	return token, nil
}

// Verify will verify the specified token and fill the specified key.
func (n *Notary) Verify(key Key, token string) error {
	// get key name
	meta := Meta(key)

	// verify token
	rawKey, err := Verify(n.secret, n.issuer, meta.Name, token)
	if err != nil {
		return err
	}

	// check id
	kid, err := coal.FromHex(rawKey.ID)
	if err != nil {
		return err
	} else if kid.IsZero() {
		return fmt.Errorf("zero key id")
	}

	// set id and expiry
	key.base().ID = kid
	key.base().Expiry = rawKey.Expiry

	// assign data
	err = jsonapi.Map(rawKey.Data).Assign(key)
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
