// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"reflect"

	"github.com/256dpi/fire/stick"
)

// Model defines the shape of a document stored in a collection. Custom types
// must implement the interface by embedding the Base type.
type Model interface {
	// GetBase returns the models base.
	GetBase() *Base

	// ID returns the primary id.
	ID() ID

	// GetAccessor should return the accessor.
	GetAccessor(interface{}) *stick.Accessor
}

// Get will lookup the specified field on the model and return its value and
// whether the field was found at all.
func Get(model Model, name string) (interface{}, bool) {
	return stick.Get(model, name)
}

// Set will set the specified field on the model with the provided value and
// return whether the field has been found and the value has been set.
func Set(model Model, name string, value interface{}) bool {
	return stick.Set(model, name, value)
}

// MustGet will call Get and panic if the operation failed.
func MustGet(model Model, name string) interface{} {
	return stick.MustGet(model, name)
}

// MustSet will call Set and panic if the operation failed.
func MustSet(model Model, name string, value interface{}) {
	stick.MustSet(model, name, value)
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

// GetAccessor implements the Model interface.
func (b *Base) GetAccessor(v interface{}) *stick.Accessor {
	return GetMeta(v.(Model)).Accessor
}

type empty struct {
	Base `bson:",inline"`
}
