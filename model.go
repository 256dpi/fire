package fire

import (
	"errors"
	"reflect"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// Model is the main interface implemented by every fire model embedding Base.
type Model interface {
	ID() bson.ObjectId
	SingularName() string
	PluralName() string
	Collection() string

	Fields() []Field
	FieldsByTag(string) []Field
	FieldWithTag(string) Field

	Get(string) interface{}
	Set(string, interface{})
	Validate(bool) error

	initialize(interface{})
}

// Init initializes the internals of a model and should be called first.
func Init(model Model) Model {
	model.initialize(model)
	return model
}

// The HasMany type denotes a has many relationship in a model declaration.
type HasMany struct{}

var supportedTags = []string{
	"filterable",
	"sortable",
	"identifiable",
	"verifiable",
	"grantable",
	"callable",
}

// A Field contains the meta information about a single field of a model.
type Field struct {
	Name     string
	JSONName string
	BSONName string
	Optional bool
	Tags     []string
	ToOne    bool
	HasMany  bool
	RelName  string
	RelType  string

	index int
}

// Base is the base for every fire model.
type Base struct {
	DocID bson.ObjectId `json:"-" bson:"_id,omitempty"`

	parentModel  interface{}
	singularName string
	pluralName   string
	collection   string
	fields       []Field
}

// ID returns the models id.
func (b *Base) ID() bson.ObjectId {
	return b.DocID
}

// SingularName returns the singular name of the model.
func (b *Base) SingularName() string {
	return b.singularName
}

// PluralName returns the plural name of the model.
func (b *Base) PluralName() string {
	return b.pluralName
}

// Collection returns the models collection.
func (b *Base) Collection() string {
	return b.collection
}

// Fields returns the models fields.
func (b *Base) Fields() []Field {
	return b.fields
}

// FieldsByTag returns all fields that contain the passed tag.
func (b *Base) FieldsByTag(tag string) []Field {
	var list []Field

	// find matching fields
	for _, field := range b.fields {
		if stringInList(field.Tags, tag) {
			list = append(list, field)
		}
	}

	return list
}

// FieldWithTag returns the first field that matches the passed tag.
//
// Note: This method panics if no field can be found.
func (b *Base) FieldWithTag(tag string) Field {
	for _, field := range b.fields {
		if stringInList(field.Tags, tag) {
			return field
		}
	}

	panic("Expected to find a field with the tag " + tag)
}

// Get returns the value of the given field.
//
// Note: Get will return the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can be found.
func (b *Base) Get(name string) interface{} {
	for _, field := range b.fields {
		if field.JSONName == name || field.BSONName == name || field.Name == name {
			// read value from model struct
			field := reflect.ValueOf(b.parentModel).Elem().Field(field.index)
			return field.Interface()
		}
	}

	panic("Missing field " + name + " on " + b.singularName)
}

// Set will set given field to the the passed valued.
//
// Note: Set will set the value of the first field that has a matching Name,
// JSONName, or BSONName and will panic if no field can been found. The method
// will also panic if the type of the field and the passed value do not match.
func (b *Base) Set(name string, value interface{}) {
	for _, field := range b.fields {
		if field.JSONName == name || field.BSONName == name || field.Name == name {
			// set the value on model struct
			reflect.ValueOf(b.parentModel).Elem().Field(field.index).Set(reflect.ValueOf(value))
			return
		}
	}

	panic("Missing field " + name + " on " + b.singularName)
}

// Validate validates the model based on the `valid:""` struct tags.
func (b *Base) Validate(fresh bool) error {
	// validate id
	if !b.DocID.Valid() {
		return errors.New("invalid id")
	}

	// validate parent model
	ok, err := govalidator.ValidateStruct(b.parentModel)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("validation failed")
	}

	return nil
}

// TODO: Speedup parsing by caching meta data.

func (b *Base) initialize(model interface{}) {
	b.parentModel = model

	// set id if missing
	if !b.DocID.Valid() {
		b.DocID = bson.NewObjectId()
	}

	// check if tags already have been parsed
	if len(b.singularName) > 0 {
		return
	}

	// get types
	baseType := reflect.TypeOf(b).Elem()
	toOneType := reflect.TypeOf(b.DocID)
	optionalToOneType := reflect.TypeOf(&b.DocID)
	hasManyType := reflect.TypeOf(HasMany{})
	modelType := reflect.TypeOf(b.parentModel).Elem()

	// iterate through all fields
	for i := 0; i < modelType.NumField(); i++ {
		structField := modelType.Field(i)

		// get fire tag
		fireStructTag := structField.Tag.Get("fire")

		// check if field is the Base
		if structField.Type == baseType {
			baseTag := strings.Split(fireStructTag, ":")
			if len(baseTag) < 2 || len(baseTag) > 3 {
				panic("Expected to find a tag of the form fire:\"singular:plural[:collection]\"")
			}

			// infer singular and plural and collection based on plural
			b.singularName = baseTag[0]
			b.pluralName = baseTag[1]
			b.collection = baseTag[1]

			// infer collection
			if len(baseTag) == 3 {
				b.collection = baseTag[2]
			}

			continue
		}

		// parse individual tags
		fireTags := strings.Split(fireStructTag, ",")
		if len(fireStructTag) == 0 {
			fireTags = nil
		}

		// prepare field
		field := Field{
			Optional: structField.Type.Kind() == reflect.Ptr,
			JSONName: getJSONFieldName(&structField),
			BSONName: getBSONFieldName(&structField),
			Name:     structField.Name,
			index:    i,
		}

		// check if field is a valid to one relationship
		if structField.Type == toOneType || structField.Type == optionalToOneType {
			if len(fireTags) > 0 && strings.Count(fireTags[0], ":") > 0 {
				if strings.Count(fireTags[0], ":") > 1 {
					panic("Expected to find a tag of the form fire:\"name:type\" on to one relationship")
				}

				// parse special to one relationship tag
				toOneTag := strings.Split(fireTags[0], ":")

				// set relationship data
				field.ToOne = true
				field.RelName = toOneTag[0]
				field.RelType = toOneTag[1]

				// remove tag
				fireTags = fireTags[1:]
			}
		}

		// check if field is a valid has many relationship
		if structField.Type == hasManyType {
			if len(fireTags) != 1 || strings.Count(fireTags[0], ":") != 1 {
				panic("Expected to find a tag of the form fire:\"name:type\" on has many relationship")
			}

			// parse special has many relationship tag
			hasManyTag := strings.Split(fireTags[0], ":")

			// set relationship data
			field.HasMany = true
			field.RelName = hasManyTag[0]
			field.RelType = hasManyTag[1]

			// remove tag
			fireTags = fireTags[1:]
		}

		// add comma separated tags
		for _, tag := range fireTags {
			if stringInList(supportedTags, tag) {
				field.Tags = append(field.Tags, tag)
			} else {
				panic("Unexpected tag: " + tag)
			}
		}

		// add field
		b.fields = append(b.fields, field)
	}
}

/* api2go.jsonapi interface */

// GetName implements the jsonapi.EntityNamer interface.
func (b *Base) GetName() string {
	return b.pluralName
}

// GetID implements the jsonapi.MarshalIdentifier interface.
func (b *Base) GetID() string {
	return b.DocID.Hex()
}

// SetID implements the jsonapi.UnmarshalIdentifier interface.
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

// GetReferences implements the jsonapi.MarshalReferences interface.
func (b *Base) GetReferences() []jsonapi.Reference {
	// prepare result
	var refs []jsonapi.Reference

	// add to one and has many relationships
	for _, field := range b.fields {
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

// GetReferencedIDs implements the jsonapi.MarshalLinkedRelations interface.
func (b *Base) GetReferencedIDs() []jsonapi.ReferenceID {
	// prepare result
	var ids []jsonapi.ReferenceID

	// add to one relationships
	for _, field := range b.fields {
		if field.ToOne {
			// get struct field
			structField := reflect.ValueOf(b.parentModel).Elem().Field(field.index)

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

// SetToOneReferenceID implements the jsonapi.UnmarshalToOneRelations interface.
func (b *Base) SetToOneReferenceID(name, id string) error {
	// check object id
	if !bson.IsObjectIdHex(id) {
		return errors.New("invalid id")
	}

	for _, field := range b.fields {
		if field.ToOne && field.RelName == name {
			// get struct field
			structField := reflect.ValueOf(b.parentModel).Elem().Field(field.index)

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
