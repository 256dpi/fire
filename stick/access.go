package stick

import (
	"fmt"
	"reflect"
	"sync"
)

// Field is dynamically accessible field.
type Field struct {
	Index int
	Type  reflect.Type
}

// Accessor provides dynamic access to a structs fields.
type Accessor struct {
	Name   string
	Fields map[string]*Field
}

// Accessible is a type that has dynamically accessible fields.
type Accessible interface {
	GetAccessor(interface{}) *Accessor
}

// GetAccessor is a short-hand to retrieve the accessor of an accessible.
func GetAccessor(acc Accessible) *Accessor {
	return acc.GetAccessor(acc)
}

var accessMutex sync.Mutex
var accessCache = map[reflect.Type]*Accessor{}

// BasicAccess may be embedded in a struct to provide basic accessibility.
type BasicAccess struct{}

// GetAccessor implements the Accessible interface.
func (a *BasicAccess) GetAccessor(v interface{}) *Accessor {
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
	accessor = BuildAccessor(v.(Accessible), "BasicAccess")

	// cache accessor
	accessCache[typ] = accessor

	return accessor
}

// BuildAccessor will build an accessor for the provided type.
func BuildAccessor(v Accessible, ignore ...string) *Accessor {
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

// Get will look up the specified field on the accessible and return its value
// and whether the field was found at all.
func Get(acc Accessible, name string) (interface{}, bool) {
	// find field
	field := GetAccessor(acc).Fields[name]
	if field == nil {
		return nil, false
	}

	// get value
	value := structValue(acc).Field(field.Index).Interface()

	return value, true
}

// GetRaw will look up the specified field on the accessible and return its raw
// value and whether the field was found at all.
func GetRaw(acc Accessible, name string) (reflect.Value, bool) {
	// find field
	field := GetAccessor(acc).Fields[name]
	if field == nil {
		return reflect.Value{}, false
	}

	// get value
	value := structValue(acc).Field(field.Index)

	return value, true
}

// Set will set the specified field on the accessible with the provided value
// and return whether the field has been found and the value has been set.
func Set(acc Accessible, name string, value interface{}) bool {
	// find field
	field := GetAccessor(acc).Fields[name]
	if field == nil {
		return false
	}

	// get value
	fieldValue := structValue(acc).Field(field.Index)

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

// MustGet will call Get and panic if the operation failed.
func MustGet(acc Accessible, name string) interface{} {
	// get value
	value, ok := Get(acc, name)
	if !ok {
		panic(fmt.Sprintf(`stick: could not get field "%s" on "%s"`, name, GetAccessor(acc).Name))
	}

	return value
}

// MustGetRaw will call GetRaw and panic if the operation failed.
func MustGetRaw(acc Accessible, name string) reflect.Value {
	// get raw value
	value, ok := GetRaw(acc, name)
	if !ok {
		panic(fmt.Sprintf(`stick: could not get field "%s" on "%s"`, name, GetAccessor(acc).Name))
	}

	return value
}

// MustSet will call Set and panic if the operation failed.
func MustSet(acc Accessible, name string, value interface{}) {
	// get value
	ok := Set(acc, name, value)
	if !ok {
		panic(fmt.Sprintf(`stick: could not set "%s" on "%s"`, name, GetAccessor(acc).Name))
	}
}

func structType(v interface{}) reflect.Type {
	typ := reflect.TypeOf(v)
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		panic("stick: expected struct")
	}
	return typ
}

func structValue(v interface{}) reflect.Value {
	val := reflect.ValueOf(v)
	for val.Type().Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		panic("stick: expected struct")
	}
	return val
}
