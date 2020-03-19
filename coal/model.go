// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"fmt"
	"reflect"
)

// Model defines the shape of a document stored in a collection. Custom types
// must implement the interface by embedding the Base type.
type Model interface {
	// ID returns the primary id.
	ID() ID

	// GetBase returns the models base.
	GetBase() *Base
}

// Get will lookup the specified field on the model and return its value and
// whether the field was found at all.
func Get(model Model, name string) (interface{}, bool) {
	// find field
	field := GetMeta(model).Fields[name]
	if field == nil {
		return nil, false
	}

	// get value
	value := reflect.ValueOf(model).Elem().Field(field.index).Interface()

	return value, true
}

// Set will set the specified field on the model with the provided value and
// return whether the field has been found and the value has been set.
func Set(model Model, name string, value interface{}) bool {
	// find field
	field := GetMeta(model).Fields[name]
	if field == nil {
		return false
	}

	// get value
	fieldValue := reflect.ValueOf(model).Elem().Field(field.index)

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
func MustGet(model Model, name string) interface{} {
	// get value
	value, ok := Get(model, name)
	if !ok {
		panic(fmt.Sprintf(`coal: could not get field "%s" on "%s"`, name, GetMeta(model).Name))
	}

	return value
}

// MustSet will call Set and panic if the operation failed.
func MustSet(model Model, name string, value interface{}) {
	// get value
	ok := Set(model, name, value)
	if !ok {
		panic(fmt.Sprintf(`coal: could not set "%s" on "%s"`, name, GetMeta(model).Name))
	}
}

// Slice takes a slice of the form *[]*Post and returns a new slice that
// contains all models.
func Slice(ptr interface{}) []Model {
	// get slice
	slice := reflect.ValueOf(ptr).Elem()

	// collect models
	models := make([]Model, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		models[i] = slice.Index(i).Interface().(Model)
	}

	return models
}

// Base is the base for every coal model.
type Base struct {
	DocID ID    `json:"-" bson:"_id,omitempty"`
	Lock  int64 `json:"-" bson:"_lk,omitempty"`
	Token ID    `json:"-" bson:"_tk,omitempty"`
}

// B is a short-hand to construct a base with the provided id or a generated
// id if none specified.
func B(id ...ID) Base {
	// check list
	if len(id) > 1 {
		panic("coal: B accepts only one id")
	}

	// use provided id id available
	if len(id) > 0 {
		return Base{
			DocID: id[0],
		}
	}

	return Base{
		DocID: New(),
	}
}

// ID implements the Model interface.
func (b *Base) ID() ID {
	return b.DocID
}

// GetBase implements the Model interface.
func (b *Base) GetBase() *Base {
	return b
}

type empty struct {
	Base `bson:",inline"`
}
