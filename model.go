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

type attribute struct {
	name         string
	index        int
	optional     bool
	filterable   bool
	sortable     bool
	identifiable bool
	verifiable   bool
	dbField      string
}

type relationship struct {
	name     string
	typ      string
	index    int
	optional bool
	dbField  string
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
}

// Collection returns the models collection.
func (b *Base) Collection() string {
	return b.collection
}

// ID returns the models id.
func (b *Base) ID() bson.ObjectId {
	return b.DocID
}

// Attribute returns the value of the given attribute.
func (b *Base) Attribute(name string) interface{} {
	b.parseTags()

	// try to find attribute in map
	attr, ok := b.attributes[name]
	if !ok {
		return nil
	}

	// get field
	field := reflect.ValueOf(b.parentModel).Elem().Field(attr.index)

	// return value
	return field.Interface()
}

// ReferenceID returns the ID of a to one relationship.
func (b *Base) ReferenceID(name string) *bson.ObjectId {
	b.parseTags()

	// try to find field in relationships map
	rel, ok := b.toOneRelationships[name]
	if !ok {
		return nil
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
	b.parseTags()
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

		// check if field is the Base
		if field.Type == baseType {
			values := strings.Split(field.Tag.Get("fire"), ":")
			if len(values) < 2 || len(values) > 3 {
				panic("expected to find a tag of the form fire:\"singular:plural[:collection]\"")
			}

			// infer singular and plural and collection based on plural
			b.singularName = values[0]
			b.pluralName = values[1]
			b.collection = values[1]

			// infer collection
			if len(values) == 3 {
				b.collection = values[2]
			}

			continue
		}

		// check if field is a to one relationship
		if field.Type == toOneType || field.Type == optionalToOneType {
			values := strings.Split(field.Tag.Get("fire"), ":")
			if len(values) == 2 {
				b.toOneRelationships[values[0]] = relationship{
					name:     values[0],
					typ:      values[1],
					index:    i,
					optional: field.Type == optionalToOneType,
					dbField:  getBSONFieldName(&field),
				}
			} else {
				panic("expected to find a tag of the form fire:\"name:type\"")
			}

			continue
		}

		// check if field is a has many relationship
		if field.Type == hasManyType {
			values := strings.Split(field.Tag.Get("fire"), ":")
			if len(values) == 2 {
				b.hasManyRelationships[values[0]] = relationship{
					name:  values[0],
					typ:   values[1],
					index: i,
				}
			} else {
				panic("expected to find a tag of the form fire:\"name:type\"")
			}

			continue
		}

		// get name of field
		name := getJSONFieldName(&field)

		// create attribute
		attr := attribute{
			name:     name,
			index:    i,
			optional: field.Type.Kind() == reflect.Ptr,
			dbField:  getBSONFieldName(&field),
		}

		// get fire tag
		tag := field.Tag.Get("fire")

		// check tags
		if len(tag) > 0 {
			for _, t := range strings.Split(tag, ",") {
				if t == "filterable" {
					attr.filterable = true
				} else if t == "sortable" {
					attr.sortable = true
				} else if t == "identifiable" {
					attr.identifiable = true
				} else if t == "verifiable" {
					attr.verifiable = true
				} else {
					panic("unexpected tag: " + t)
				}
			}
		}

		// add attribute
		b.attributes[name] = attr
	}
}

/* api2go.jsonapi interface */

// GetName implements the jsonapi.EntityNamer interface.
func (b *Base) GetName() string {
	b.parseTags()
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
	b.parseTags()

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
	b.parseTags()

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
	b.parseTags()

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
