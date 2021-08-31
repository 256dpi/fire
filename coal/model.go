// Package coal provides a mini ORM for mongoDB.
package coal

import (
	"reflect"

	"github.com/256dpi/fire/stick"
)

// Model defines the shape of a document stored in a collection. Custom types
// must implement the interface by embedding the Base type.
type Model interface {
	ID() ID
	Validate() error
	GetBase() *Base
	GetAccessor(interface{}) *stick.Accessor
}

// Base is the base for every coal model.
type Base struct {
	DocID ID      `json:"-" bson:"_id,omitempty"`
	Lock  int64   `json:"-" bson:"_lk,omitempty"`
	Token ID      `json:"-" bson:"_tk,omitempty"`
	Score float64 `json:"-" bson:"_sc,omitempty"`
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

// Slice takes a slice of the form []Post, []*Post, *[]Post or *[]*Post and
// returns a new slice that contains all models.
func Slice(val interface{}) []Model {
	// get slice
	slice := reflect.ValueOf(val)
	if slice.Kind() == reflect.Ptr {
		slice = slice.Elem()
	}

	// collect models
	models := make([]Model, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		model := slice.Index(i)
		if model.Kind() == reflect.Struct {
			model = model.Addr()
		}
		models[i] = model.Interface().(Model)
	}

	return models
}
