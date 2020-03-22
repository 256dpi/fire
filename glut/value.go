package glut

import (
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Value is a structure used to encode a value.
type Value interface {
	GetBase() *Base
	GetAccessor(v interface{}) *stick.Accessor
}

// ExtendedValue is a value that can extends its key.
type ExtendedValue interface {
	Value
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

// GetAccessor implements the Value interface.
func (b *Base) GetAccessor(v interface{}) *stick.Accessor {
	return GetMeta(v.(Value)).Accessor
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a value.
type Meta struct {
	// The values type.
	Type reflect.Type

	// The values key.
	Key string

	// The values time to live.
	TTL time.Duration

	// The used transfer coding.
	Coding coal.Coding

	// The accessor.
	Accessor *stick.Accessor
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}

// GetMeta will parse the values "glut" tag on the embedded glut.Base struct and
// return the meta object.
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

	// check coding tag
	json, hasJSON := field.Tag.Lookup("json")
	bson, hasBSON := field.Tag.Lookup("bson")
	if (hasJSON && hasBSON) || (!hasJSON && !hasBSON) {
		panic(`glut: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "glut.Base"`)
	} else if (hasJSON && json != "-") || (hasBSON && bson != "-") {
		panic(`glut: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "glut.Base"`)
	}

	// get coding
	coding := coal.JSON
	if hasBSON {
		coding = coal.BSON
	}

	// split tag
	tag := strings.Split(field.Tag.Get("glut"), ",")

	// check tag
	if len(tag) != 2 || tag[0] == "" || tag[1] == "" {
		panic(`glut: expected to find a tag of the form 'glut:"key,ttl"' on "glut.Base"`)
	}

	// get key
	key := tag[0]

	// get ttl
	ttl, err := time.ParseDuration(tag[1])
	if err != nil {
		panic(`glut: invalid duration as time to live on "glut.Base"`)
	}

	// prepare meta
	meta := &Meta{
		Type:     typ,
		Key:      key,
		TTL:      ttl,
		Coding:   coding,
		Accessor: stick.BuildAccessor(value, "Base"),
	}

	// cache meta
	metaCache[typ] = meta

	return meta
}
