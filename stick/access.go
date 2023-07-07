package stick

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/256dpi/lungo/bsonkit"
)

var accessMutex sync.Mutex
var accessCache = map[reflect.Type]*Accessor{}

// Accessible is a type that provides a custom accessor for dynamic access.
type Accessible interface {
	GetAccessor(interface{}) *Accessor
}

// Field is a dynamically accessible field.
type Field struct {
	Index int
	Type  reflect.Type
}

// Accessor provides dynamic access to a structs fields.
type Accessor struct {
	Name   string
	Fields map[string]*Field
}

// Access will create, cache and return an accessor for the provided value.
//
// Note: Ignored fields are only applied the very first time Access is called
// for the provided type.
func Access(v interface{}, ignore ...string) *Accessor {
	// get type
	typ := structType(v)

	// acquire mutex
	accessMutex.Lock()
	defer accessMutex.Unlock()

	// check if accessor has already been cached
	accessor, ok := accessCache[typ]
	if ok {
		return accessor
	}

	// build accessor
	accessor = BuildAccessor(v, ignore...)

	// cache accessor
	accessCache[typ] = accessor

	return accessor
}

// BuildAccessor will build and return an accessor for the provided type.
func BuildAccessor(v interface{}, ignore ...string) *Accessor {
	// get type
	typ := structType(v)

	// prepare accessor
	accessor := &Accessor{
		Name:   typ.String(),
		Fields: map[string]*Field{},
	}

	// iterate through all fields
	for i := 0; i < typ.NumField(); i++ {
		// get field
		field := typ.Field(i)

		// check field
		var skip bool
		for _, item := range ignore {
			if item == field.Name {
				skip = true
			}
		}
		if skip {
			continue
		}

		// add field
		accessor.Fields[field.Name] = &Field{
			Index: i,
			Type:  field.Type,
		}
	}

	return accessor
}

// GetAccessor is a shorthand to retrieve a value's accessor.
func GetAccessor(v interface{}) *Accessor {
	// check type
	structType(v)

	// check if accessible
	if acc, ok := v.(Accessible); ok {
		return acc.GetAccessor(v)
	}

	// otherwise, get accessor on demand
	return Access(v)
}

// Get will call GetRaw and return the interface value.
func Get(v interface{}, path string) (interface{}, bool) {
	// get raw value
	value, ok := GetRaw(v, path)
	if !ok {
		return nil, false
	}

	return value.Interface(), true
}

// MustGet will call MustGetRaw and return the interface value.
func MustGet(v interface{}, path string) interface{} {
	return MustGetRaw(v, path).Interface()
}

// GetRaw will look up and return the value of the specified field and whether
// the field was found at all.
func GetRaw(v interface{}, path string) (reflect.Value, bool) {
	// get first name
	name := bsonkit.PathSegment(path)

	// find field
	field := GetAccessor(v).Fields[name]
	if field == nil {
		return reflect.Value{}, false
	}

	// get value
	value := structValue(v).Field(field.Index)

	for {
		// reduce path and return on end
		path = bsonkit.ReducePath(path)
		if path == bsonkit.PathEnd {
			return value, true
		}

		// get next name
		name = bsonkit.PathSegment(path)

		// handle list index
		num, index := bsonkit.ParseIndex(name)
		if index {
			// check slice
			if value.Kind() != reflect.Slice {
				return reflect.Value{}, false
			} else if num >= value.Len() {
				return reflect.Value{}, false
			}

			// set value
			value = value.Index(num)

			continue
		}

		// ensure pointer to value
		if value.Kind() != reflect.Ptr {
			value = value.Addr()
		}

		// descend
		return GetRaw(value.Interface(), path)
	}
}

// MustGetRaw will call GetRaw and panic if the operation failed.
func MustGetRaw(v interface{}, name string) reflect.Value {
	// get raw value
	value, ok := GetRaw(v, name)
	if !ok {
		panic(fmt.Sprintf(`stick: could not get field "%s" on "%s"`, name, GetAccessor(v).Name))
	}

	return value
}

// Set will set the specified field with the provided value and return whether
// the field has been found and the value has been set.
func Set(v interface{}, path string, value interface{}) bool {
	// get first name
	name := bsonkit.PathSegment(path)

	// find field
	field := GetAccessor(v).Fields[name]
	if field == nil {
		return false
	}

	// get value
	fieldValue := structValue(v).Field(field.Index)

	for {
		// reduce path and break on end
		path = bsonkit.ReducePath(path)
		if path == bsonkit.PathEnd {
			break
		}

		// get next name
		name = bsonkit.PathSegment(path)

		// handle list index
		num, index := bsonkit.ParseIndex(name)
		if index {
			// check slice
			if fieldValue.Kind() != reflect.Slice {
				return false
			} else if num >= fieldValue.Len() {
				return false
			}

			// set value
			fieldValue = fieldValue.Index(num)

			continue
		}

		// ensure pointer to value
		if fieldValue.Kind() != reflect.Ptr {
			fieldValue = fieldValue.Addr()
		}

		// descend
		return Set(fieldValue.Interface(), path, value)
	}

	/* set new value */

	// get value value
	valueValue := reflect.ValueOf(value)

	// correct untyped nil values
	if value == nil && field.Type.Kind() == reflect.Ptr {
		valueValue = reflect.Zero(field.Type)
	}

	// check type
	if fieldValue.Type() != valueValue.Type() {
		return false
	}

	// set value
	fieldValue.Set(valueValue)

	return true
}

// MustSet will call Set and panic if the operation failed.
func MustSet(v interface{}, name string, value interface{}) {
	// get value
	ok := Set(v, name, value)
	if !ok {
		panic(fmt.Sprintf(`stick: could not set "%s" on "%s"`, name, GetAccessor(v).Name))
	}
}

func structType(v interface{}) reflect.Type {
	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Ptr {
		panic("stick: expected pointer")
	}
	typ = typ.Elem()
	if typ.Kind() != reflect.Struct {
		panic("stick: expected struct")
	}
	return typ
}

func structValue(v interface{}) reflect.Value {
	val := reflect.ValueOf(v)
	if val.Type().Kind() != reflect.Ptr {
		panic("stick: expected pointer")
	}
	if val.IsNil() {
		panic("stick: nil pointer")
	}
	val = val.Elem()
	if val.Kind() != reflect.Struct {
		panic("stick: expected struct")
	}
	return val
}
