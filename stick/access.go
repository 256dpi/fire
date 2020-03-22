package stick

import (
	"fmt"
	"reflect"
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

// Get will lookup the specified field on the accessible and return its value
// and whether the field was found at all.
func Get(acc Accessible, name string) (interface{}, bool) {
	// find field
	field := acc.GetAccessor(acc).Fields[name]
	if field == nil {
		return nil, false
	}

	// get value
	value := reflect.ValueOf(acc).Elem().Field(field.Index).Interface()

	return value, true
}

// Set will set the specified field on the accessible with the provided value
// and return whether the field has been found and the value has been set.
func Set(acc Accessible, name string, value interface{}) bool {
	// find field
	field := acc.GetAccessor(acc).Fields[name]
	if field == nil {
		return false
	}

	// get value
	fieldValue := reflect.ValueOf(acc).Elem().Field(field.Index)

	// get value value
	valueValue := reflect.ValueOf(value)

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
		panic(fmt.Sprintf(`stick: could not get field "%s" on "%s"`, name, acc.GetAccessor(acc).Name))
	}

	return value
}

// MustSet will call Set and panic if the operation failed.
func MustSet(acc Accessible, name string, value interface{}) {
	// get value
	ok := Set(acc, name, value)
	if !ok {
		panic(fmt.Sprintf(`stick: could not set "%s" on "%s"`, name, acc.GetAccessor(acc).Name))
	}
}
