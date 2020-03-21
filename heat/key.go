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
	// GetBase should be implemented by embedding Base.
	GetBase() *Base

	// Validate should validate the token.
	Validate() error
}

// Base can be embedded in a struct to turn it into a key.
type Base struct {
	ID     coal.ID
	Expiry time.Time
}

// GetBase implements the Key interface.
func (b *Base) GetBase() *Base {
	return b
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a key.
type Meta struct {
	Name   string
	Expiry time.Duration
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}
var metaNames = map[string]reflect.Type{}

// GetMeta will parse the keys "heat" tag on the embedded heat.Base struct and
// return the encoded name and default expiry.
func GetMeta(key Key) *Meta {
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
	meta := &Meta{
		Name:   name,
		Expiry: expiry,
	}

	// cache meta
	metaCache[typ] = meta

	// flag name
	metaNames[name] = typ

	return meta
}
