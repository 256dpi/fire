// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"fmt"
	"reflect"
)

// Model is the main interface implemented by every coal model embedding Base.
type Model interface {
	ID() ID
	Meta() *Meta

	initialize(Model)
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

// Init initializes the internals of a model and should be called before using
// a newly created Model.
func Init(model Model) Model {
	model.initialize(model)
	return model
}

// InitSlice initializes all models in a slice of the form *[]*Post and returns
// a new slice that contains all initialized models.
func InitSlice(ptr interface{}) []Model {
	// get slice
	slice := reflect.ValueOf(ptr).Elem()

	// make model slice
	models := make([]Model, slice.Len())

	// iterate over entries
	for i := 0; i < slice.Len(); i++ {
		m := Init(slice.Index(i).Interface().(Model))
		models[i] = m
	}

	return models
}

// Base is the base for every coal model.
type Base struct {
	DocID ID `json:"-" bson:"_id"`

	model Model
	meta  *Meta
}

// ID returns the models id.
func (b *Base) ID() ID {
	return b.DocID
}

// Meta returns the models Meta structure.
func (b *Base) Meta() *Meta {
	return b.meta
}

func (b *Base) initialize(model Model) {
	b.model = model

	// set id if missing
	if b.DocID.IsZero() {
		b.DocID = New()
	}

	// assign meta
	b.meta = GetMeta(model)
}
