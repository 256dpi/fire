package nitro

import (
	"reflect"
	"strings"
	"sync"

	"github.com/256dpi/fire/stick"
)

// Procedure denotes types that can be processed by the BSON-RPC system.
type Procedure interface {
	Validate() error
	GetBase() *Base
	GetAccessor(interface{}) *stick.Accessor
}

// Base can be embedded in a struct to turn it into a procedure.
type Base struct {
	// Response is set to true if the procedure is treated as a response.
	Response bool
}

// GetBase implements the Key interface.
func (b *Base) GetBase() *Base {
	return b
}

// GetAccessor implements the Key interface.
func (b *Base) GetAccessor(v interface{}) *stick.Accessor {
	return GetMeta(v.(Procedure)).Accessor
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a procedure.
type Meta struct {
	// The values type.
	Type reflect.Type

	// The name.
	Name string

	// The used transfer coding.
	Coding stick.Coding

	// The accessor.
	Accessor *stick.Accessor
}

var metaCache = map[reflect.Type]*Meta{}
var metaMutex sync.Mutex

// GetMeta will parse the keys "nitro" tag on the embedded nitro.Base struct and
// return the meta object.
func GetMeta(proc Procedure) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get type
	typ := reflect.TypeOf(proc).Elem()

	// check cache
	meta, ok := metaCache[typ]
	if ok {
		return meta
	}

	// get first field
	field := typ.Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`nitro: expected first struct field to be an embedded "nitro.Base"`)
	}

	// check coding tag
	json, hasJSON := field.Tag.Lookup("json")
	bson, hasBSON := field.Tag.Lookup("bson")
	if (hasJSON && hasBSON) || (!hasJSON && !hasBSON) {
		panic(`nitro: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "nitro.Base"`)
	} else if (hasJSON && json != "-") || (hasBSON && bson != "-") {
		panic(`nitro: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "nitro.Base"`)
	}

	// get coding
	coding := stick.JSON
	if hasBSON {
		coding = stick.BSON
	}

	// split tag
	tag := strings.Split(field.Tag.Get("nitro"), ",")

	// check tag
	if len(tag) != 1 || tag[0] == "" {
		panic(`nitro: expected to find a tag of the form 'nitro:"name"' on "nitro.Base"`)
	}

	// get names
	name := strings.Trim(tag[0], "/")

	// prepare meta
	meta = &Meta{
		Type:     typ,
		Name:     name,
		Coding:   coding,
		Accessor: stick.BuildAccessor(proc, "Base"),
	}

	// cache meta
	metaCache[typ] = meta

	return meta
}

// Make creates a new instance of the procedure.
func (i *Meta) Make() Procedure {
	return reflect.New(i.Type).Interface().(Procedure)
}
