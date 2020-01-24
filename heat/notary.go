package heat

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/256dpi/jsonapi/v2"

	"github.com/256dpi/fire/coal"
)

// Key is a structure used to encode a key.
type Key interface {
	// Validate should validate the token.
	Validate() error

	base() *Base
}

// Base can be embedded in a struct to turn it into a key.
type Base struct {
	ID     coal.ID
	Expiry time.Time
}

func (b *Base) base() *Base {
	return b
}

var baseType = reflect.TypeOf(Base{})

type meta struct {
	name   string
	expiry time.Duration
}

var metaCache = map[reflect.Type]meta{}
var metaMutex sync.Mutex

// Meta will parse the keys "heat" tag on the embedded heat.Base struct and
// return the encoded name and default expiry.
func Meta(key Key) (string, time.Duration) {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(key)

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta.name, meta.expiry
	}

	// get first field
	field := typ.Elem().Field(0)

	// check field type
	if field.Type != baseType {
		panic(`heat: expected first struct field to be of type "heat.Base"`)
	}

	// check field name
	if field.Name != "Base" {
		panic(`heat: expected an embedded "heat.Base" as the first struct field`)
	}

	// split tag
	tag := strings.Split(field.Tag.Get("heat"), ",")

	// check tag
	if len(tag) != 2 || tag[0] == "" || tag[1] == "" {
		panic(`heat: expected to find a tag of the form 'heat:"name,expiry"' on "heat.Base"`)
	}

	// get expiry
	expiry, err := time.ParseDuration(tag[1])
	if err != nil {
		panic(err)
	}

	// cache meta
	metaCache[typ] = meta{
		name:   tag[0],
		expiry: expiry,
	}

	return tag[0], expiry
}

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

	// set random secret if missing
	if len(secret) == 0 {
		panic("heat: missing secret")
	}

	// check secret
	if len(secret) < 16 {
		panic("heat: secret too short")
	}

	return &Notary{
		secret: secret,
		issuer: name,
	}
}

// Issue will generate a token from the specified key.
func (n *Notary) Issue(key Key) (string, error) {
	// get key name and default expiry
	name, expiry := Meta(key)

	// get base
	base := key.base()

	// ensure id
	if base.ID.IsZero() {
		base.ID = coal.New()
	}

	// ensure expiry
	if base.Expiry.IsZero() {
		base.Expiry = time.Now().Add(expiry)
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
	token, err := Issue(n.secret, n.issuer, name, RawKey{
		ID:     base.ID.Hex(),
		Expiry: time.Now().Add(expiry),
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
	name, _ := Meta(key)

	// verify token
	rawKey, err := Verify(n.secret, n.issuer, name, token)
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
