// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"fmt"
	"reflect"

	"gopkg.in/mgo.v2/bson"
)

// Model is the main interface implemented by every coal model embedding Base.
type Model interface {
	ID() bson.ObjectId
	Meta() *Meta

	MustGet(string) interface{}
	MustSet(string, interface{})

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

// Base is the base for every coal model.
type Base struct {
	DocID bson.ObjectId `json:"-" bson:"_id"`

	model interface{}
	meta  *Meta
}

// ID returns the models id.
func (b *Base) ID() bson.ObjectId {
	return b.DocID
}

// MustGet returns the value of the given field. MustGet will panic if no field
// has been found.
func (b *Base) MustGet(name string) interface{} {
	// find field
	field := b.meta.Fields[name]
	if field == nil {
		panic(fmt.Sprintf(`coal: field "%s" not found on "%s"`, name, b.meta.Name))
	}

	// read value from model struct
	structField := reflect.ValueOf(b.model).Elem().Field(field.index)
	return structField.Interface()
}

// MustSet will set the given field to the the passed valued. MustSet will panic
// if no field has been found.
func (b *Base) MustSet(name string, value interface{}) {
	// find field
	field := b.meta.Fields[name]
	if field == nil {
		panic(fmt.Sprintf(`coal: field "%s" not found on "%s"`, name, b.meta.Name))
	}

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
