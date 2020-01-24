package heat

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

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

// KeyMeta contains meta information about a key.
type KeyMeta struct {
	Name   string
	Expiry time.Duration
}

var metaCache = map[reflect.Type]KeyMeta{}
var metaNames = map[string]reflect.Type{}
var metaMutex sync.Mutex

// Meta will parse the keys "heat" tag on the embedded heat.Base struct and
// return the encoded name and default expiry.
func Meta(key Key) KeyMeta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(key)

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta
	}

	// get first field
	field := typ.Elem().Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`heat: expected first struct field to be an embedded "heat.Base"`)
	}

	// check json tag
	if field.Tag.Get("json") != "-" {
		panic(`heat: expected to find a tag of the form 'json:"-"' on "heat.Base"`)
	}

	// split tag
	tag := strings.Split(field.Tag.Get("heat"), ",")

	// check tag
	if len(tag) != 2 || tag[0] == "" || tag[1] == "" {
		panic(`heat: expected to find a tag of the form 'heat:"name,expiry"' on "heat.Base"`)
	}

	// get name
	name := tag[0]
	if existing := metaNames[name]; existing != nil {
		panic(fmt.Sprintf(`heat: key name %q has already been registered by type %q`, name, existing.String()))
	}

	// get expiry
	expiry, err := time.ParseDuration(tag[1])
	if err != nil {
		panic(`heat: invalid duration as expiry on "heat.Base"`)
	}

	// prepare meta
	meta := KeyMeta{
		Name:   name,
		Expiry: expiry,
	}

	// cache meta
	metaCache[typ] = meta

	// flag name
	metaNames[name] = typ

	return meta
}
