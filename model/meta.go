package model

import (
	"reflect"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

var metaCache = make(map[string]*Meta)

var baseType = reflect.TypeOf(Base{})
var toOneType = reflect.TypeOf(bson.ObjectId(""))
var optionalToOneType = reflect.TypeOf(new(bson.ObjectId))
var toManyType = reflect.TypeOf(make([]bson.ObjectId, 0))
var hasManyType = reflect.TypeOf(HasMany{})

// The HasMany type denotes a has many relationship in a model declaration.
type HasMany struct{}

// A Field contains the meta information about a single field of a model.
type Field struct {
	Name       string
	Type       reflect.Kind
	JSONName   string
	BSONName   string
	Optional   bool
	Filterable bool
	Sortable   bool
	ToOne      bool
	ToMany     bool
	HasMany    bool
	RelName    string
	RelType    string
	RelInverse string

	index int
}

// Meta stores extracted meta data from a model.
type Meta struct {
	Name       string
	PluralName string
	Collection string
	Fields     []Field

	model Model
}

// NewMeta returns the Meta structure for the passed Model.
//
// Note: This method panics if the passed Model has invalid fields and tags.
func NewMeta(model Model) *Meta {
	// get type and name
	modelType := reflect.TypeOf(model).Elem()
	modelName := modelType.String()

	// check if meta has already been cached
	meta, ok := metaCache[modelName]
	if ok {
		return meta
	}

	// create new meta
	meta = &Meta{
		Name:  modelName,
		model: model,
	}

	// iterate through all fields
	for i := 0; i < modelType.NumField(); i++ {
		structField := modelType.Field(i)

		// get fire tag
		fireStructTag := structField.Tag.Get("fire")

		// check if field is the Base
		if structField.Type == baseType {
			baseTag := strings.Split(fireStructTag, ":")

			// check json tag
			if structField.Tag.Get("json") != "-" {
				panic(`Expected to find a tag of the form json:"-" on Base`)
			}

			// check bson tag
			if structField.Tag.Get("bson") != ",inline" {
				panic(`Expected to find a tag of the form bson:",inline" on Base`)
			}

			// check tag
			if len(baseTag) > 2 || baseTag[0] == "" {
				panic(`Expected to find a tag of the form fire:"plural-name[:collection]" on Base`)
			}

			// infer plural and collection names
			meta.PluralName = baseTag[0]
			meta.Collection = baseTag[0]

			// infer collection
			if len(baseTag) == 2 {
				meta.Collection = baseTag[1]
			}

			continue
		}

		// parse individual tags
		fireTags := strings.Split(fireStructTag, ",")
		if len(fireStructTag) == 0 {
			fireTags = nil
		}

		// get field type
		fieldType := structField.Type.Kind()
		if fieldType == reflect.Ptr {
			fieldType = structField.Type.Elem().Kind()
		}

		// prepare field
		field := Field{
			Name:     structField.Name,
			Type:     fieldType,
			JSONName: getJSONFieldName(&structField),
			BSONName: getBSONFieldName(&structField),
			Optional: structField.Type.Kind() == reflect.Ptr,
			index:    i,
		}

		// check if field is a valid to one relationship
		if structField.Type == toOneType || structField.Type == optionalToOneType {
			if len(fireTags) > 0 && strings.Count(fireTags[0], ":") > 0 {
				// check tag
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

		// check if field is a valid to many relationship
		if structField.Type == toManyType {
			if len(fireTags) > 0 && strings.Count(fireTags[0], ":") > 0 {
				// check tag
				if strings.Count(fireTags[0], ":") > 1 {
					panic("Expected to find a tag of the form fire:\"name:type\" on to many relationship")
				}

				// parse special to many relationship tag
				toManyTag := strings.Split(fireTags[0], ":")

				// set relationship data
				field.ToMany = true
				field.RelName = toManyTag[0]
				field.RelType = toManyTag[1]

				// remove tag
				fireTags = fireTags[1:]
			}
		}

		// check if field is a valid has many relationship
		if structField.Type == hasManyType {
			// check tag
			if len(fireTags) != 1 || strings.Count(fireTags[0], ":") != 2 {
				panic(`Expected to find a tag of the form fire:"name:type:inverse" on has many relationship`)
			}

			// parse special has many relationship tag
			hasManyTag := strings.Split(fireTags[0], ":")

			// set relationship data
			field.HasMany = true
			field.RelName = hasManyTag[0]
			field.RelType = hasManyTag[1]
			field.RelInverse = hasManyTag[2]

			// remove tag
			fireTags = fireTags[1:]
		}

		// add comma separated tags
		for _, tag := range fireTags {
			if tag == "filterable" {
				field.Filterable = true
			} else if tag == "sortable" {
				field.Sortable = true
			} else {
				panic("Unexpected tag " + tag)
			}
		}

		// add field
		meta.Fields = append(meta.Fields, field)
	}

	// cache meta
	metaCache[modelName] = meta

	return meta
}

// Make returns a pointer to a new zero initialized model e.g. *Post.
//
// Note: Other libraries like mgo might replace the pointer content with a new
// structure, therefore the model eventually needs to be initialized again
// using Init().
func (m *Meta) Make() Model {
	pointer := reflect.New(reflect.TypeOf(m.model).Elem()).Interface()
	return Init(pointer.(Model))
}

// MakeSlice returns a pointer to a zero length slice of the model e.g. *[]*Post.
//
// Note: Don't forget to initialize the slice using InitSlice() after adding
// elements with libraries like mgo.
func (m *Meta) MakeSlice() interface{} {
	slice := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(m.model)), 0, 0)
	pointer := reflect.New(slice.Type())
	pointer.Elem().Set(slice)
	return pointer.Interface()
}

func getJSONFieldName(field *reflect.StructField) string {
	tag := field.Tag.Get("json")
	values := strings.Split(tag, ",")

	// check for "-"
	if tag == "-" {
		return ""
	}

	// check first value
	if len(tag) > 0 || len(values[0]) > 0 {
		return values[0]
	}

	return field.Name
}

func getBSONFieldName(field *reflect.StructField) string {
	tag := field.Tag.Get("bson")
	values := strings.Split(tag, ",")

	// check for "-"
	if tag == "-" {
		return ""
	}

	// check first value
	if len(tag) > 0 || len(values[0]) > 0 {
		return values[0]
	}

	return strings.ToLower(field.Name)
}
