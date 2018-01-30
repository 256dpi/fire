package coal

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
var hasOneType = reflect.TypeOf(HasOne{})
var hasManyType = reflect.TypeOf(HasMany{})

// The HasOne type denotes a has-one relationship in a model declaration.
//
// Has-one relationships requires that the referencing side is ensuring that the
// reference is unique. In fire this should be done using a uniqueness validator
// and a unique index on the collection.
type HasOne struct{}

// The HasMany type denotes a has-many relationship in a model declaration.
type HasMany struct{}

// A Field contains the meta information about a single field of a model.
type Field struct {
	Name       string
	Type       reflect.Type
	Kind       reflect.Kind
	JSONName   string
	BSONName   string
	Optional   bool
	ToOne      bool
	ToMany     bool
	HasOne     bool
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
		field := modelType.Field(i)

		// get coal tag
		coalTag := field.Tag.Get("coal")

		// check if field is the Base
		if field.Type == baseType {
			baseTag := strings.Split(coalTag, ":")

			// check json tag
			if field.Tag.Get("json") != "-" {
				panic(`coal: expected to find a tag of the form json:"-" on Base`)
			}

			// check bson tag
			if field.Tag.Get("bson") != ",inline" {
				panic(`coal: expected to find a tag of the form bson:",inline" on Base`)
			}

			// check valid tag
			if field.Tag.Get("valid") != "required" {
				panic(`coal: expected to find a tag of the form valid:"required" on Base`)
			}

			// check tag
			if len(baseTag) > 2 || baseTag[0] == "" {
				panic(`coal: expected to find a tag of the form coal:"plural-name[:collection]" on Base`)
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
		coalTags := strings.Split(coalTag, ",")
		if len(coalTag) == 0 {
			coalTags = nil
		}

		// get field type
		fieldKind := field.Type.Kind()
		if fieldKind == reflect.Ptr {
			fieldKind = field.Type.Elem().Kind()
		}

		// prepare field
		metaField := Field{
			Name:     field.Name,
			Type:     field.Type,
			Kind:     fieldKind,
			JSONName: getJSONFieldName(&field),
			BSONName: getBSONFieldName(&field),
			Optional: field.Type.Kind() == reflect.Ptr,
			index:    i,
		}

		// check if field is a valid to-one relationship
		if field.Type == toOneType || field.Type == optionalToOneType {
			if len(coalTags) > 0 && strings.Count(coalTags[0], ":") > 0 {
				// check valid tag
				if !strings.Contains(field.Tag.Get("valid"), "object-id") {
					panic(`coal: missing "object-id" validation on to-one relationship`)
				}

				// check tag
				if strings.Count(coalTags[0], ":") > 1 {
					panic(`coal: expected to find a tag of the form coal:"name:type" on to-one relationship`)
				}

				// parse special to-one relationship tag
				toOneTag := strings.Split(coalTags[0], ":")

				// set relationship data
				metaField.ToOne = true
				metaField.RelName = toOneTag[0]
				metaField.RelType = toOneTag[1]

				// remove tag
				coalTags = coalTags[1:]
			}
		}

		// check if field is a valid to-many relationship
		if field.Type == toManyType {
			if len(coalTags) > 0 && strings.Count(coalTags[0], ":") > 0 {
				// check valid tag
				if !strings.Contains(field.Tag.Get("valid"), "object-id") {
					panic(`coal: missing "object-id" validation on to-many relationship`)
				}

				// check tag
				if strings.Count(coalTags[0], ":") > 1 {
					panic(`coal: expected to find a tag of the form coal:"name:type" on to-many relationship`)
				}

				// parse special to-many relationship tag
				toManyTag := strings.Split(coalTags[0], ":")

				// set relationship data
				metaField.ToMany = true
				metaField.RelName = toManyTag[0]
				metaField.RelType = toManyTag[1]

				// remove tag
				coalTags = coalTags[1:]
			}
		}

		// check if field is a valid has-one relationship
		if field.Type == hasOneType {
			// check tag
			if len(coalTags) != 1 || strings.Count(coalTags[0], ":") != 2 {
				panic(`coal: expected to find a tag of the form coal:"name:type:inverse" on has-one relationship`)
			}

			// parse special has-one relationship tag
			hasOneTag := strings.Split(coalTags[0], ":")

			// set relationship data
			metaField.HasOne = true
			metaField.RelName = hasOneTag[0]
			metaField.RelType = hasOneTag[1]
			metaField.RelInverse = hasOneTag[2]

			// remove tag
			coalTags = coalTags[1:]
		}

		// check if field is a valid has-many relationship
		if field.Type == hasManyType {
			// check tag
			if len(coalTags) != 1 || strings.Count(coalTags[0], ":") != 2 {
				panic(`coal: expected to find a tag of the form coal:"name:type:inverse" on has-many relationship`)
			}

			// parse special has-many relationship tag
			hasManyTag := strings.Split(coalTags[0], ":")

			// set relationship data
			metaField.HasMany = true
			metaField.RelName = hasManyTag[0]
			metaField.RelType = hasManyTag[1]
			metaField.RelInverse = hasManyTag[2]

			// remove tag
			coalTags = coalTags[1:]
		}

		// panic on any additional tags
		for _, tag := range coalTags {
			panic("coal: unexpected tag " + tag)
		}

		// add field
		meta.Fields = append(meta.Fields, metaField)
	}

	// cache meta
	metaCache[modelName] = meta

	return meta
}

// FindField returns the first field that has a matching Name. FindField will
// return nil if no field has been found.
func (m *Meta) FindField(name string) *Field {
	for _, field := range m.Fields {
		if field.Name == name {
			return &field
		}
	}

	return nil
}

// MustFindField returns the first field that has a matching Name. MustFindField
// will panic if no field has been found.
func (m *Meta) MustFindField(name string) *Field {
	field := m.FindField(name)
	if field == nil {
		panic(`coal: field "` + name + `" not found on "` + m.Name + `"`)
	}

	return field
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
