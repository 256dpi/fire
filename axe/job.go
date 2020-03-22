package axe

import (
	"reflect"
	"strings"
	"sync"

	"github.com/256dpi/fire/coal"
)

// Job is a structure used to encode a job.
type Job interface {
	ID() coal.ID
	GetBase() *Base
}

// Base can be embedded in a struct to turn it into a job.
type Base struct {
	// The id of the document.
	DocID coal.ID

	// The label of the job.
	Label string
}

// B is a short-hand to construct a base with a label.
func B(label string) Base {
	return Base{
		Label: label,
	}
}

// ID will return the jobs id.
func (b *Base) ID() coal.ID {
	return b.DocID
}

// GetBase implements the Job interface.
func (b *Base) GetBase() *Base {
	return b
}

var baseType = reflect.TypeOf(Base{})

// Meta contains meta information about a job.
type Meta struct {
	// The jobs type.
	Type reflect.Type

	// The jobs name.
	Name string

	// The used transfer coding.
	Coding coal.Coding
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}

// GetMeta will parse the jobs "axe" tag on the embedded axe.Base struct and
// return the meta object.
func GetMeta(job Job) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(job).Elem()

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta
	}

	// get first field
	field := typ.Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`axe: expected first struct field to be an embedded "axe.Base"`)
	}

	// check coding tag
	json, hasJSON := field.Tag.Lookup("json")
	bson, hasBSON := field.Tag.Lookup("bson")
	if (hasJSON && hasBSON) || (!hasJSON && !hasBSON) {
		panic(`axe: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "axe.Base"`)
	} else if (hasJSON && json != "-") || (hasBSON && bson != "-") {
		panic(`axe: expected to find a coding tag of the form 'json:"-"' or 'bson:"-"' on "axe.Base"`)
	}

	// get coding
	coding := coal.JSON
	if hasBSON {
		coding = coal.BSON
	}

	// split tag
	tag := strings.Split(field.Tag.Get("axe"), ",")

	// check tag
	if len(tag) != 1 || tag[0] == "" {
		panic(`axe: expected to find a tag of the form 'axe:"name"' on "axe.Base"`)
	}

	// get name
	name := tag[0]

	// prepare meta
	meta := &Meta{
		Type:   typ,
		Name:   name,
		Coding: coding,
	}

	// cache meta
	metaCache[typ] = meta

	return meta
}

// Make returns a pointer to a new zero initialized job e.g. *Increment.
func (m *Meta) Make() Job {
	return reflect.New(m.Type).Interface().(Job)
}
