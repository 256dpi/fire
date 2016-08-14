package fire

import (
	"errors"
	"reflect"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// TODO: Is it really a model?

// Model is the main interface implemented by every fire model embedding Base.
type Model interface {
	ID() bson.ObjectId
	Collection() string
	Attribute(string) interface{}
	SetAttribute(string, interface{})
	Validate(bool) error

	getBase() *Base
	initialize(interface{})
}

// Init initializes the internals of a model and should be called first.
func Init(model Model) Model {
	model.initialize(model)
	return model
}

var errInvalidID = errors.New("invalid id")

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

func (b *Base) initialize(model interface{}) {
	b.parentModel = model

	// set id if missing
	if !b.DocID.Valid() {
		b.DocID = bson.NewObjectId()
	}

	b.parseTags()
}

// ID returns the models id.
func (b *Base) ID() bson.ObjectId {
	return b.DocID
}

// Collection returns the models collection.
func (b *Base) Collection() string {
	return b.collection
}

// Attribute returns the value of the given attribute.
//
// Note: Attribute will return the first attribute that has a matching JSON,
// BSON or struct field name and will panic if no attribute can be found.
func (b *Base) Attribute(name string) interface{} {
	// try to find attribute in map
	for _, attr := range b.fields {
		if attr.JSONName == name || attr.BSONName == name || attr.Name == name {
			// read value from model struct
			field := reflect.ValueOf(b.parentModel).Elem().Field(attr.index)
			return field.Interface()
		}
	}

	panic(b.singularName + ": missing attribute " + name)
}

// SetAttribute will set given attribute to the the passed valued.
//
// Note: SetAttribute will set the first attribute that has a matching JSON,
// BSON or struct field name and will panic if none has been found. The method
// will also panic if the type of the attribute and the passed value do not match.
func (b *Base) SetAttribute(name string, value interface{}) {
	// try to find attribute in map
	for _, attr := range b.fields {
		if attr.JSONName == name || attr.BSONName == name || attr.Name == name {
			// set the value on model struct
			reflect.ValueOf(b.parentModel).Elem().Field(attr.index).Set(reflect.ValueOf(value))
			return
		}
	}

	panic(b.singularName + ":missing attribute " + name)
}

// Validate validates the model based on the `valid:""` struct tags.
func (b *Base) Validate(fresh bool) error {
	// validate id
	if !b.DocID.Valid() {
		return errInvalidID
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

func (b *Base) getBase() *Base {
	return b
}

func (b *Base) parseTags() {
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
		field := modelType.Field(i)

		// get fire tag
		fireStructTag := field.Tag.Get("fire")

		// check if field is the Base
		if field.Type == baseType {
			baseTag := strings.Split(fireStructTag, ":")
			if len(baseTag) < 2 || len(baseTag) > 3 {
				panic("expected to find a tag of the form fire:\"singular:plural[:collection]\"")
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

		// prepare attribute
		attr := Field{
			Optional: field.Type.Kind() == reflect.Ptr,
			JSONName: getJSONFieldName(&field),
			BSONName: getBSONFieldName(&field),
			Name:     field.Name,
			index:    i,
		}

		// check if field is a valid to one relationship
		if field.Type == toOneType || field.Type == optionalToOneType {
			if len(fireTags) > 0 && strings.Count(fireTags[0], ":") > 0 {
				if strings.Count(fireTags[0], ":") > 1 {
					panic("Expected to find a tag of the form fire:\"name:type\" on to one relationship")
				}

				// parse special to one relationship tag
				toOneTag := strings.Split(fireTags[0], ":")

				// set attributes
				attr.ToOne = true
				attr.RelName = toOneTag[0]
				attr.RelType = toOneTag[1]

				// remove tag
				fireTags = fireTags[1:]
			}
		}

		// check if field is a valid has many relationship
		if field.Type == hasManyType {
			if len(fireTags) != 1 || strings.Count(fireTags[0], ":") != 1 {
				panic("Expected to find a tag of the form fire:\"name:type\" on has many relationship")
			}

			// parse special has many relationship tag
			hasManyTag := strings.Split(fireTags[0], ":")

			// set attributes
			attr.HasMany = true
			attr.RelName = hasManyTag[0]
			attr.RelType = hasManyTag[1]

			// remove tag
			fireTags = fireTags[1:]
		}

		// add comma separated tags
		for _, tag := range fireTags {
			if stringInList(supportedTags, tag) {
				attr.Tags = append(attr.Tags, tag)
			} else {
				panic("unexpected tag: " + tag)
			}
		}

		// add attribute
		b.fields = append(b.fields, attr)
	}
}

func (b *Base) attributesByTag(tag string) []Field {
	var list []Field

	// find identifiable and verifiable attributes
	for _, attr := range b.fields {
		if stringInList(attr.Tags, tag) {
			list = append(list, attr)
		}
	}

	return list
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
		return errInvalidID
	}

	b.DocID = bson.ObjectIdHex(id)
	return nil
}

// GetReferences implements the jsonapi.MarshalReferences interface.
func (b *Base) GetReferences() []jsonapi.Reference {
	// prepare result
	var refs []jsonapi.Reference

	// add to one and has many relationships
	for _, attr := range b.fields {
		if attr.ToOne || attr.HasMany {
			refs = append(refs, jsonapi.Reference{
				Type:        attr.RelType,
				Name:        attr.RelName,
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
	for _, attr := range b.fields {
		if attr.ToOne {
			field := reflect.ValueOf(b.parentModel).Elem().Field(attr.index)

			// prepare id
			var id string

			// check if field is optional
			if attr.Optional {
				// continue if id is not set
				if field.IsNil() {
					continue
				}

				// get id
				id = field.Interface().(*bson.ObjectId).Hex()
			} else {
				// get id
				id = field.Interface().(bson.ObjectId).Hex()
			}

			// append reference id
			ids = append(ids, jsonapi.ReferenceID{
				ID:   id,
				Type: attr.RelType,
				Name: attr.RelName,
			})
		}
	}

	return ids
}

// SetToOneReferenceID implements the jsonapi.UnmarshalToOneRelations interface.
func (b *Base) SetToOneReferenceID(name, id string) error {
	// check object id
	if !bson.IsObjectIdHex(id) {
		return errInvalidID
	}

	for _, attr := range b.fields {
		if attr.ToOne && attr.RelName == name {
			// get field
			field := reflect.ValueOf(b.parentModel).Elem().Field(attr.index)

			// create id
			oid := bson.ObjectIdHex(id)

			// check if optional
			if attr.Optional {
				field.Set(reflect.ValueOf(&oid))
			} else {
				field.Set(reflect.ValueOf(oid))
			}

			return nil
		}
	}

	return errors.New("missing relationship " + name)
}

// TODO: Implement jsonapi.UnmarshalToManyRelations interface.
