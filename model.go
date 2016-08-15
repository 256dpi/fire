package fire

import (
	"errors"
	"reflect"

	"github.com/asaskevich/govalidator"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// Model is the main interface implemented by every fire model embedding Base.
type Model interface {
	ID() bson.ObjectId
	Get(string) interface{}
	Set(string, interface{})
	Validate(bool) error
	Meta() *Meta

	initialize(Model)
}

// Init initializes the internals of a model and should be called first.
func Init(model Model) Model {
	model.initialize(model)
	return model
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

	panic("Missing field " + name + " on " + b.meta.SingularName)
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

	panic("Missing field " + name + " on " + b.meta.SingularName)
}

// Validate validates the model based on the `valid:""` struct tags.
func (b *Base) Validate(fresh bool) error {
	// validate id
	if !b.DocID.Valid() {
		return errors.New("invalid id")
	}

	// validate parent model
	ok, err := govalidator.ValidateStruct(b.model)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("validation failed")
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

/* api2go.jsonapi interface */

// GetName returns the plural name of the Model.
//
// This methods is required by the jsonapi.EntityNamer interface.
func (b *Base) GetName() string {
	return b.meta.PluralName
}

// GetID returns the id of the Model.
//
// This methods is required by the jsonapi.MarshalIdentifier interface.
func (b *Base) GetID() string {
	return b.DocID.Hex()
}

// SetID sets the id of the Model.
//
// This methods is required by the jsonapi.UnmarshalIdentifier interface.
func (b *Base) SetID(id string) error {
	if len(id) == 0 {
		b.DocID = bson.NewObjectId()
		return nil
	}

	if !bson.IsObjectIdHex(id) {
		return errors.New("invalid id")
	}

	b.DocID = bson.ObjectIdHex(id)
	return nil
}

// GetReferences returns a list of the available references.
//
// This methods is required by the jsonapi.MarshalReferences interface.
func (b *Base) GetReferences() []jsonapi.Reference {
	// prepare result
	var refs []jsonapi.Reference

	// add to one and has many relationships
	for _, field := range b.meta.Fields {
		if field.ToOne || field.HasMany {
			refs = append(refs, jsonapi.Reference{
				Type:        field.RelType,
				Name:        field.RelName,
				IsNotLoaded: true,
			})
		}
	}

	return refs
}

// GetReferencesIDs returns list of references ids.
//
// This methods is required by the jsonapi.MarshalLinkedRelations interface.
func (b *Base) GetReferencedIDs() []jsonapi.ReferenceID {
	// prepare result
	var ids []jsonapi.ReferenceID

	// add to one relationships
	for _, field := range b.meta.Fields {
		if field.ToOne {
			// get struct field
			structField := reflect.ValueOf(b.model).Elem().Field(field.index)

			// prepare id
			var id string

			// check if field is optional
			if field.Optional {
				// continue if id is not set
				if structField.IsNil() {
					continue
				}

				// get id
				id = structField.Interface().(*bson.ObjectId).Hex()
			} else {
				// get id
				id = structField.Interface().(bson.ObjectId).Hex()
			}

			// append reference id
			ids = append(ids, jsonapi.ReferenceID{
				ID:   id,
				Type: field.RelType,
				Name: field.RelName,
			})
		}
	}

	return ids
}

// SetToOneReferenceID sets a reference to the passed id.
//
// This methods is required by the jsonapi.UnmarshalToOneRelations interface.
func (b *Base) SetToOneReferenceID(name, id string) error {
	// check object id
	if !bson.IsObjectIdHex(id) {
		return errors.New("invalid id")
	}

	for _, field := range b.meta.Fields {
		if field.ToOne && field.RelName == name {
			// get struct field
			structField := reflect.ValueOf(b.model).Elem().Field(field.index)

			// create id
			oid := bson.ObjectIdHex(id)

			// check if optional
			if field.Optional {
				structField.Set(reflect.ValueOf(&oid))
			} else {
				structField.Set(reflect.ValueOf(oid))
			}

			return nil
		}
	}

	return errors.New("missing relationship " + name)
}
