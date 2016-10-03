// Package model implements a basic model abstraction for structs.
package model

import (
	"reflect"

	"gopkg.in/mgo.v2/bson"
)

// Model is the main interface implemented by every fire model embedding Base.
type Model interface {
	ID() bson.ObjectId
	MustGet(string) interface{}
	MustSet(string, interface{})
	Meta() *Meta

	initialize(Model)
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

// Base is the base for every fire model.
type Base struct {
	DocID bson.ObjectId `json:"-" bson:"_id,omitempty"`

	model interface{}
	meta  *Meta
}

// ID returns the models id.
func (b *Base) ID() bson.ObjectId {
	return b.DocID
}

// MustGet returns the value of the given field.
//
// Note: MustGet will return the value of the first field that has a matching
// Name, JSONName, or BSONName and will panic if no field can be found.
func (b *Base) MustGet(name string) interface{} {
	field := b.meta.MustFindField(name)

	// read value from model struct
	structField := reflect.ValueOf(b.model).Elem().Field(field.index)
	return structField.Interface()
}

// MustSet will set the given field to the the passed valued.
//
// Note: MustSet will set the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can been found. The method
// will also panic if the type of the field and the passed value do not match.
func (b *Base) MustSet(name string, value interface{}) {
	field := b.meta.MustFindField(name)

	// set the value on model struct
	reflect.ValueOf(b.model).Elem().Field(field.index).Set(reflect.ValueOf(value))
}

// Meta returns the models Meta structure.
func (b *Base) Meta() *Meta {
	return b.meta
}

func (b *Base) initialize(model Model) {
	b.model = model

	// set id if missing
	if !b.DocID.Valid() {
		b.DocID = bson.NewObjectId()
	}

	// assign meta
	b.meta = NewMeta(model)
}
