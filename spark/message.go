package spark

import (
	"reflect"
	"strings"
	"sync"

	"github.com/256dpi/fire/stick"
)

// Message is a structure used to encode a message.
type Message interface {
	Validate() error
	GetBase() *Base
	GetAccessor(interface{}) *stick.Accessor
}

// Base can be embedded in a struct to turn it into a message.
type Base struct{}

// GetBase implements the Message interface.
func (b *Base) GetBase() *Base {
	return b
}

// GetAccessor implements the Message interface.
func (b *Base) GetAccessor(v interface{}) *stick.Accessor {
	return GetMeta(v.(Message)).Accessor
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a message.
type Meta struct {
	// The type.
	Type reflect.Type

	// The messages name.
	Name string

	// The used transfer coding.
	Coding stick.Coding

	// The accessor.
	Accessor *stick.Accessor
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}

// GetMeta will parse the messages "spark" tag on the embedded spark.Base struct
// and return the encoded name.
func GetMeta(message Message) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(message)

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta
	}

	// get first field
	field := typ.Elem().Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`spark: expected first struct field to be an embedded "spark.Base"`)
	}

	// check coding tag
	json, hasJSON := field.Tag.Lookup("json")
	bson, hasBSON := field.Tag.Lookup("bson")
	if (hasJSON && hasBSON) || (!hasJSON && !hasBSON) {
		panic(`spark: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "spark.Base"`)
	} else if (hasJSON && json != "-") || (hasBSON && bson != "-") {
		panic(`spark: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "spark.Base"`)
	}

	// get coding
	coding := stick.JSON
	if hasBSON {
		coding = stick.BSON
	}

	// split tag
	tag := strings.Split(field.Tag.Get("spark"), ",")

	// check tag
	if len(tag) != 1 || tag[0] == "" {
		panic(`spark: expected to find a tag of the form 'spark:"name"' on "spark.Base"`)
	}

	// get name
	name := tag[0]

	// prepare meta
	meta := &Meta{
		Type:     typ,
		Name:     name,
		Coding:   coding,
		Accessor: stick.BuildAccessor(message, "Base"),
	}

	// cache meta
	metaCache[typ] = meta

	return meta
}
