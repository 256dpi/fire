package kiln

import (
	"reflect"
	"strings"
	"sync"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// Process is a structure used to encode a process.
type Process interface {
	ID() coal.ID
	Validate() error
	GetBase() *Base
	GetAccessor(interface{}) *stick.Accessor
}

// Base can be embedded in a struct to turn it into a process.
type Base struct {
	// The id of the document.
	DocID coal.ID

	// The label of the process.
	Label string
}

// B is a short-hand to construct a base with a label.
func B(label string) Base {
	return Base{
		Label: label,
	}
}

// ID will return the processes id.
func (b *Base) ID() coal.ID {
	return b.DocID
}

// GetBase implements the Process interface.
func (b *Base) GetBase() *Base {
	return b
}

// GetAccessor implements the Model interface.
func (b *Base) GetAccessor(v interface{}) *stick.Accessor {
	return GetMeta(v.(Process)).Accessor
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a process.
type Meta struct {
	// The process type.
	Type reflect.Type

	// The process name.
	Name string

	// The used transfer coding.
	Coding stick.Coding

	// The accessor.
	Accessor *stick.Accessor
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}

// GetMeta will parse the process "kiln" tag on the embedded kiln.Base struct
// and return the meta object.
func GetMeta(proc Process) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(proc).Elem()

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta
	}

	// get first field
	field := typ.Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`kiln: expected first struct field to be an embedded "kiln.Base"`)
	}

	// check coding tag
	json, hasJSON := field.Tag.Lookup("json")
	bson, hasBSON := field.Tag.Lookup("bson")
	if (hasJSON && hasBSON) || (!hasJSON && !hasBSON) {
		panic(`kiln: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "kiln.Base"`)
	} else if (hasJSON && json != "-") || (hasBSON && bson != "-") {
		panic(`kiln: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "kiln.Base"`)
	}

	// get coding
	coding := stick.JSON
	if hasBSON {
		coding = stick.BSON
	}

	// split tag
	tag := strings.Split(field.Tag.Get("kiln"), ",")

	// check tag
	if len(tag) != 1 || tag[0] == "" {
		panic(`kiln: expected to find a tag of the form 'kiln:"name"' on "kiln.Base"`)
	}

	// get name
	name := tag[0]

	// prepare meta
	meta := &Meta{
		Type:     typ,
		Name:     name,
		Coding:   coding,
		Accessor: stick.BuildAccessor(proc, "Base"),
	}

	// cache meta
	metaCache[typ] = meta

	return meta
}

// Make returns a pointer to a new zero initialized process e.g. *Calculator.
func (m *Meta) Make() Process {
	return reflect.New(m.Type).Interface().(Process)
}
