package fire

import (
	"errors"
	"reflect"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/manyminds/api2go/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

var ErrInvalidID = errors.New("invalid id")

// The HasMany type denotes a has many relationship in a model declaration.
type HasMany struct{}

type relationship struct {
	name     string
	typ      string
	index    int
	optional bool
}

// Base is the base for every fire model.
type Base struct {
	ID bson.ObjectId `json:"-" bson:"_id,omitempty"`

	parentModel          interface{}
	singularName         string
	pluralName           string
	toOneRelationships   map[string]relationship
	hasManyRelationships map[string]relationship
}

func (b *Base) initialize(model interface{}) {
	b.parentModel = model

	// set id if missing
	if !b.ID.Valid() {
		b.ID = bson.NewObjectId()
	}
}

func (b *Base) parseTags() {
	// check if tags already have been parsed
	if len(b.singularName) > 0 {
		return
	}

	// prepare storage
	b.toOneRelationships = make(map[string]relationship)
	b.hasManyRelationships = make(map[string]relationship)

	// get types
	baseType := reflect.TypeOf(b).Elem()
	toOneType := reflect.TypeOf(b.ID)
	optionalToOneType := reflect.TypeOf(&b.ID)
	hasManyType := reflect.TypeOf(HasMany{})
	modelType := reflect.TypeOf(b.parentModel).Elem()

	// iterate through all fields
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// check if field is the Base
		if field.Type == baseType {
			values := strings.Split(field.Tag.Get("fire"), ":")
			if len(values) == 2 {
				b.singularName = values[0]
				b.pluralName = values[1]
			} else {
				panic("expected to find a tag of the form fire:\"singular:plural\"")
			}
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
				}
			}
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
			}
		}
	}
}

func (b *Base) getSingularName() string {
	b.parseTags()
	return b.singularName
}

func (b *Base) getObjectID() bson.ObjectId {
	return b.ID
}

// Validate validates the parent model.
func (b *Base) Validate(fresh bool) error {
	// validate id
	if !b.ID.Valid() {
		return ErrInvalidID
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

/* api2go.jsonapi interfaces */

// GetName returns the resource name.
func (b *Base) GetName() string {
	b.parseTags()
	return b.pluralName
}

// GetID returns the id as a string.
func (b *Base) GetID() string {
	return b.ID.Hex()
}

// SetID sets the supplied string id.
func (b *Base) SetID(id string) error {
	if len(id) == 0 {
		b.ID = bson.NewObjectId()
		return nil
	}

	if !bson.IsObjectIdHex(id) {
		return ErrInvalidID
	}

	b.ID = bson.ObjectIdHex(id)
	return nil
}

// GetReferences returns a list of possible relations.
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

// GetReferencedIDs returns a list of references.
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

// GetToOneReferenceID returns a relation.
func (b *Base) GetToOneReferenceID(name string) (string, error) {
	b.parseTags()

	// try to find field in relationships map
	rel, ok := b.toOneRelationships[name]
	if !ok {
		return "", errors.New("missing relationship " + name)
	}

	// get field
	field := reflect.ValueOf(b.parentModel).Elem().Field(rel.index)

	// check if field is optional
	if rel.optional {
		// return empty id if not set
		if field.IsNil() {
			return "", nil
		}

		// return id
		return field.Elem().Interface().(bson.ObjectId).Hex(), nil
	}

	// return id
	return field.Interface().(bson.ObjectId).Hex(), nil
}

// SetToOneReferenceID saves a relation.
func (b *Base) SetToOneReferenceID(name, id string) error {
	b.parseTags()

	// check object id
	if !bson.IsObjectIdHex(id) {
		return ErrInvalidID
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
