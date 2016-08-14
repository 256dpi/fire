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
	Collection() string
	Attribute(string) interface{}
	SetAttribute(string, interface{})
	ReferenceID(string) *bson.ObjectId
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

type attribute struct {
	jsonName  string
	bsonName  string
	fieldName string
	tags      []string
	index     int
}

type relationship struct {
	name      string
	bsonName  string
	fieldName string
	typ       string
	optional  bool
	index     int
}

// Base is the base for every fire model.
type Base struct {
	DocID bson.ObjectId `json:"-" bson:"_id,omitempty"`

	parentModel          interface{}
	singularName         string
	pluralName           string
	collection           string
	attributes           map[string]attribute
	toOneRelationships   map[string]relationship
	hasManyRelationships map[string]relationship
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
	for _, attr := range b.attributes {
		if attr.jsonName == name || attr.bsonName == name || attr.fieldName == name {
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
	for _, attr := range b.attributes {
		if attr.jsonName == name || attr.bsonName == name || attr.fieldName == name {
			// set the value on model struct
			reflect.ValueOf(b.parentModel).Elem().Field(attr.index).Set(reflect.ValueOf(value))
			return
		}
	}

	panic(b.singularName + ":missing attribute " + name)
}

// ReferenceID returns the ID of a to one relationship.
//
// Note: ReferenceID will panic if the relationship does not exist.
func (b *Base) ReferenceID(name string) *bson.ObjectId {
	// try to find field in relationships map
	rel, ok := b.toOneRelationships[name]
	if !ok {
		panic(b.singularName + ": missing to one relationship " + name)
	}

	// get field
	field := reflect.ValueOf(b.parentModel).Elem().Field(rel.index)

	// check if field is optional
	if rel.optional {
		// return empty id if not set
		if field.IsNil() {
			return nil
		}

		// return id
		return field.Interface().(*bson.ObjectId)
	}

	// return id
	id := field.Interface().(bson.ObjectId)
	return &id
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

	// prepare storage
	b.attributes = make(map[string]attribute)
	b.toOneRelationships = make(map[string]relationship)
	b.hasManyRelationships = make(map[string]relationship)

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

		// get field names
		jsonName := getJSONFieldName(&field)
		bsonName := getBSONFieldName(&field)

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

		// check if field is marked as a ToOne relationship
		if field.Type == toOneType || field.Type == optionalToOneType {
			if len(fireTags) > 0 && strings.Count(fireTags[0], ":") > 0 {
				if strings.Count(fireTags[0], ":") > 1 {
					panic("Expected to find a tag of the form fire:\"name:type\" on ToOne relationship")
				}

				toOneTag := strings.Split(fireTags[0], ":")
				b.toOneRelationships[toOneTag[0]] = relationship{
					name:      toOneTag[0],
					bsonName:  bsonName,
					fieldName: field.Name,
					typ:       toOneTag[1],
					optional:  field.Type == optionalToOneType,
					index:     i,
				}

				continue
			}
		}

		// check if field is marked as a HasMany relationship
		if field.Type == hasManyType {
			if len(fireTags) != 1 || strings.Count(fireTags[0], ":") != 1 {
				panic("Expected to find a tag of the form fire:\"name:type\" on HasMany relationship")
			}

			hasManyTag := strings.Split(fireTags[0], ":")
			b.hasManyRelationships[hasManyTag[0]] = relationship{
				name:      hasManyTag[0],
				fieldName: field.Name,
				typ:       hasManyTag[1],
				index:     i,
			}

			continue
		}

		// create attribute
		attr := attribute{
			jsonName:  jsonName,
			bsonName:  bsonName,
			fieldName: field.Name,
			index:     i,
		}

		// check if optional
		if field.Type.Kind() == reflect.Ptr {
			attr.tags = append(attr.tags, "optional")
		}

		// add tags
		for _, tag := range fireTags {
			if stringInList(supportedTags, tag) {
				attr.tags = append(attr.tags, tag)
			} else {
				panic("unexpected tag: " + tag)
			}
		}

		// add attribute
		b.attributes[field.Name] = attr
	}
}

func (b *Base) attributesByTag(tag string) []attribute {
	var list []attribute

	// find identifiable and verifiable attributes
	for _, attr := range b.attributes {
		if stringInList(attr.tags, tag) {
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

	// add to one relationships
	for _, rel := range b.toOneRelationships {
		refs = append(refs, jsonapi.Reference{
			Type:        rel.typ,
			Name:        rel.name,
			IsNotLoaded: true,
		})
	}

	// add has many relationships
	for _, rel := range b.hasManyRelationships {
		refs = append(refs, jsonapi.Reference{
			Type:        rel.typ,
			Name:        rel.name,
			IsNotLoaded: true,
		})
	}

	return refs
}

// GetReferencedIDs implements the jsonapi.MarshalLinkedRelations interface.
func (b *Base) GetReferencedIDs() []jsonapi.ReferenceID {
	// prepare result
	var ids []jsonapi.ReferenceID

	// add to one relationships
	for _, rel := range b.toOneRelationships {
		field := reflect.ValueOf(b.parentModel).Elem().Field(rel.index)

		// prepare id
		var id string

		// check if field is optional
		if rel.optional {
			// continue if id is not set
			if field.IsNil() {
				continue
			}

			// get id
			id = field.Elem().Interface().(bson.ObjectId).Hex()
		} else {
			// get id
			id = field.Interface().(bson.ObjectId).Hex()
		}

		// append reference id
		ids = append(ids, jsonapi.ReferenceID{
			ID:   id,
			Type: rel.typ,
			Name: rel.name,
		})
	}

	return ids
}

// SetToOneReferenceID implements the jsonapi.UnmarshalToOneRelations interface.
func (b *Base) SetToOneReferenceID(name, id string) error {
	// check object id
	if !bson.IsObjectIdHex(id) {
		return errInvalidID
	}

	// try to find field in relationships map
	rel, ok := b.toOneRelationships[name]
	if !ok {
		return errors.New("missing relationship " + name)
	}

	// get field
	field := reflect.ValueOf(b.parentModel).Elem().Field(rel.index)

	// create id
	oid := bson.ObjectIdHex(id)

	// check if optional
	if rel.optional {
		field.Set(reflect.ValueOf(&oid))
	} else {
		field.Set(reflect.ValueOf(oid))
	}

	return nil
}

// TODO: Implement jsonapi.UnmarshalToManyRelations interface.
