package axe

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Job is a structure used to encode a job.
type Job interface {
	GetBase() *Base
}

// Base can be embedded in a struct to turn it into a job.
type Base struct{}

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
}

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}
var metaNames = map[string]reflect.Type{}

// GetMeta will parse the jobs "axe" tag on the embedded axe.Base struct and
// return the meta object.
func GetMeta(job Job) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get typ
	typ := reflect.TypeOf(job)

	// check cache
	if meta, ok := metaCache[typ]; ok {
		return meta
	}

	// get first field
	field := typ.Elem().Field(0)

	// check field type and name
	if field.Type != baseType || !field.Anonymous || field.Name != "Base" {
		panic(`axe: expected first struct field to be an embedded "axe.Base"`)
	}

	// check json tag
	if field.Tag.Get("json") != "-" {
		panic(`axe: expected to find a tag of the form 'json:"-"' on "axe.Base"`)
	}

	// split tag
	tag := strings.Split(field.Tag.Get("axe"), ",")

	// check tag
	if len(tag) != 1 || tag[0] == "" {
		panic(`axe: expected to find a tag of the form 'axe:"name"' on "axe.Base"`)
	}

	// get name
	name := tag[0]
	if existing := metaNames[name]; existing != nil {
		panic(fmt.Sprintf(`axe: job name %q has already been registered by type %q`, name, existing.String()))
	}

	// prepare meta
	meta := &Meta{
		Type: typ,
		Name: name,
	}

	// cache meta
	metaCache[typ] = meta

	// flag name
	metaNames[name] = typ

	return meta
}
