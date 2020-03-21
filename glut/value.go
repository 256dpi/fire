package glut

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/256dpi/fire/coal"
)

// Value is a structure used to encode a value.
type Value interface {
	// GetBase should be implemented by embedding Base.
	GetBase() *Base
}

// ExtendedValue is a value that can extends its key.
type ExtendedValue interface {
	GetExtension() (string, error)
}

// Base can be embedded in a struct to turn it into a value.
type Base struct {
	// The token used for value locking.
	Token coal.ID
}

// GetBase implements the Value interface.
func (b *Base) GetBase() *Base {
	return b
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a value.
type Meta struct {
	// The type of the value.
	Type reflect.Type

	// The values key.
	Key string

	// The values time to live.
	TTL time.Duration
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}
var metaKeys = map[string]reflect.Type{}

// GetMeta will parse the values "glut" tag on the embedded glut.Base struct and
// return the encoded component and name.
func GetMeta(value Value) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(value)

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta
	}

	// get first field
	field := typ.Elem().Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`glut: expected first struct field to be an embedded "glut.Base"`)
	}

	// check json tag
	if field.Tag.Get("json") != "-" {
		panic(`glut: expected to find a tag of the form 'json:"-"' on "glut.Base"`)
	}

	// split tag
	tag := strings.Split(field.Tag.Get("glut"), ",")

	// check tag
	if len(tag) != 2 || tag[0] == "" || tag[1] == "" {
		panic(`glut: expected to find a tag of the form 'glut:"key,ttl"' on "glut.Base"`)
	}

	// get key
	key := tag[0]
	if existing := metaKeys[key]; existing != nil {
		panic(fmt.Sprintf(`glut: value key %q has already been registered by type %q`, key, existing.String()))
	}

	// get ttl
	ttl, err := time.ParseDuration(tag[1])
	if err != nil {
		panic(`glut: invalid duration as time to live on "glut.Base"`)
	}

	// prepare meta
	meta := &Meta{
		Type: typ,
		Key:  key,
		TTL:  ttl,
	}

	// cache meta
	metaCache[typ] = meta

	// flag key
	metaKeys[key] = typ

	return meta
}
