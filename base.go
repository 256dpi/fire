package fire

import (
	"errors"
	"reflect"

	"github.com/asaskevich/govalidator"
	"gopkg.in/mgo.v2/bson"
)

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

// Get returns the value of the given field.
//
// Note: Get will return the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can be found.
func (b *Base) Get(name string) interface{} {
	for _, field := range b.meta.Fields {
		if field.JSONName == name || field.BSONName == name || field.Name == name {
			// read value from model struct
			field := reflect.ValueOf(b.model).Elem().Field(field.index)
			return field.Interface()
		}
	}

	panic("Missing field " + name + " on model named " + b.meta.PluralName)
}

// Set will set given field to the the passed valued.
//
// Note: Set will set the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can been found. The method
// will also panic if the type of the field and the passed value do not match.
func (b *Base) Set(name string, value interface{}) {
	for _, field := range b.meta.Fields {
		if field.JSONName == name || field.BSONName == name || field.Name == name {
			// set the value on model struct
			reflect.ValueOf(b.model).Elem().Field(field.index).Set(reflect.ValueOf(value))
			return
		}
	}

	panic("Missing field " + name + " on model named " + b.meta.PluralName)
}

// Validate validates the model based on the "valid" struct tags.
func (b *Base) Validate(fresh bool) error {
	// validate id
	if !b.DocID.Valid() {
		return errors.New("Invalid id")
	}

	// validate parent model
	_, err := govalidator.ValidateStruct(b.model)
	if err != nil {
		return err
	}

	return nil
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
