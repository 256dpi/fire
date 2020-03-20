package glut

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Value is a structure used to encode a value.
type Value interface {
	// GetBase should be implemented by embedding Base.
	GetBase() *Base
}

// Base can be embedded in a struct to turn it into a value.
type Base struct {}

// GetBase implements the Value interface.
func (b *Base) GetBase() *Base {
	return b
}

var baseType = reflect.TypeOf(Base{})

// ValueMeta contains meta information about a value.
type ValueMeta struct {
	Key string
	TTL time.Duration
}

var metaCache = map[reflect.Type]ValueMeta{}
var metaKeys = map[string]reflect.Type{}
var metaMutex sync.Mutex

// Meta will parse the values "glut" tag on the embedded glut.Base struct and
// return the encoded component and name.
func Meta(value Value) ValueMeta {
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
		panic(`glut: invalid duration as TTL on "glut.Base"`)
	}

	// prepare meta
	meta := ValueMeta{
		Key: key,
		TTL: ttl,
	}

	// cache meta
	metaCache[typ] = meta

	// flag key
	metaKeys[key] = typ

	return meta
}
