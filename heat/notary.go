package heat

import (
	"context"
	"time"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// TODO: Allow secret data to be provided with a stored key.
//  => Would allow us to use this system for handling invitations.
//  => Unique IDs/Labels?

var ErrMissingKey = xo.BF("missing key")

// Notary is used to issue and verify tokens from keys.
type Notary struct {
	issuer string
	secret []byte
	store  *coal.Store
}

// NewNotary creates a new notary with the specified name and secret. It will
// panic if the name is missing or the specified secret is less than 16 bytes.
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
		issuer: name,
		secret: secret,
	}
}

// Issue will generate a token from the specified key. If store is true, the
// key is also saved in the specified store and flagged respectively.
func (n *Notary) Issue(ctx context.Context, key Key, store bool) (string, error) {
	// trace
	ctx, span := xo.Trace(ctx, "heat/Notary.Issue")
	span.Tag("store", store)
	defer span.End()

	// get meta
	meta := GetMeta(key)

	// get base
	base := key.GetBase()

	// ensure id
	if base.ID.IsZero() {
		base.ID = coal.New()
	}

	// ensure issued
	if base.Issued.IsZero() {
		base.Issued = time.Now()
	}

	// ensure expires
	if base.Expires.IsZero() {
		base.Expires = base.Issued.Add(meta.Expiry)
	}

	// validate key
	err := key.Validate()
	if err != nil {
		return "", err
	}

	// get data
	var data stick.Map
	err = data.Marshal(key, stick.JSON)
	if err != nil {
		return "", err
	}

	// TODO: Set stored flag in claims?

	// issue token
	token, err := Issue(n.secret, n.issuer, meta.Name, RawKey{
		ID:      base.ID.Hex(),
		Issued:  base.Issued,
		Expires: base.Expires,
		Data:    data,
	})
	if err != nil {
		return "", err
	}

	// return token if not stored
	if !store {
		return token, nil
	}

	// TODO: Set parent.

	// otherwise, prepare model
	model := Model{
		Base:    coal.B(base.ID),
		Issued:  base.Issued,
		Expires: base.Expires,
		Data:    data,
	}

	// insert key
	err = n.store.M(&model).Insert(ctx, &model)
	if err != nil {
		return "", err
	}

	return token, nil
}

// TODO: IssueAndStore()?

// Verify will verify the specified token and fill the specified key. If retrieve
// is true, the key is retrieve from the store and verified to exist and not
// be revoked.
func (n *Notary) Verify(ctx context.Context, key Key, token string, retrieve bool) error {
	// trace
	ctx, span := xo.Trace(ctx, "heat/Notary.Verify")
	defer span.End()

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
		return xo.F("invalid token id")
	}

	// set base
	*key.GetBase() = Base{
		ID:      kid,
		Issued:  rawKey.Issued,
		Expires: rawKey.Expires,
	}

	// assign data
	err = rawKey.Data.Unmarshal(key, stick.JSON)
	if err != nil {
		return err
	}

	// validate key
	err = key.Validate()
	if err != nil {
		return err
	}

	// return if not retrieve
	if !retrieve {
		return nil
	}

	// otherwise, retrieve key
	var model Model
	found, err := n.store.M(&model).Find(ctx, &model, key.GetBase().ID, false)
	if err != nil {
		return err
	} else if !found {
		return ErrMissingKey.Wrap()
	}

	// TODO: Further validate?

	return nil
}

// TODO: VerifyAndRetrieve()?

// Revoke will revoke the specified key by deleting it.
func (n *Notary) Revoke(ctx context.Context, key Key) error {
	// trace
	ctx, span := xo.Trace(ctx, "heat/Notary.Delete")
	defer span.End()

	// TODO: Also revoke descendants?

	// delete key
	found, err := n.store.M(&Model{}).Delete(ctx, nil, key.GetBase().ID)
	if err != nil {
		return err
	} else if !found {
		return ErrMissingKey.Wrap()
	}

	return nil
}
