package coal

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

var metaCache sync.Map

var baseType = reflect.TypeOf(Base{})
var toOneType = reflect.TypeOf(New())
var optionalToOneType = reflect.TypeOf(new(ID))
var toManyType = reflect.TypeOf(make([]ID, 0))
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
	// The struct field name e.g. "TireSize".
	Name string

	// The struct field type and kind.
	Type reflect.Type
	Kind reflect.Kind

	// The JSON object key name e.g. "tire-size".
	JSONKey string

	// The BSON document field e.g. "tire_size".
	BSONField string

	// The custom flags.
	Flags []string

	// Whether the field is a pointer and thus optional.
	Optional bool

	// The relationship status.
	ToOne   bool
	ToMany  bool
	HasOne  bool
	HasMany bool

	// The relationship information.
	RelName    string
	RelType    string
	RelInverse string

	index int
}

// Meta stores extracted meta data from a model.
type Meta struct {
	// The struct type.
	Type reflect.Type

	// The struct type name e.g. "models.CarWheel".
	Name string

	// The plural resource name e.g. "car-wheels".
	PluralName string

	// The collection name e.g. "car_wheels".
	Collection string

	// The struct fields.
	Fields map[string]*Field

	// The struct fields ordered.
	OrderedFields []*Field

	// The database fields.
	DatabaseFields map[string]*Field

	// The attributes.
	Attributes map[string]*Field

	// The relationships.
	Relationships map[string]*Field

	// The flagged fields.
	FlaggedFields map[string][]*Field
}

// GetMeta returns the Meta structure for the passed Model.
//
// Note: This method panics if the passed Model has invalid fields and tags.
func GetMeta(model Model) *Meta {
	// get type and name
	modelType := reflect.TypeOf(model).Elem()

	// check if meta has already been cached
	value, ok := metaCache.Load(modelType)
	if ok {
		return value.(*Meta)
	}

	// create new meta
	meta := &Meta{
		Type:           modelType,
		Name:           modelType.String(),
		Fields:         make(map[string]*Field),
		DatabaseFields: make(map[string]*Field),
		Attributes:     make(map[string]*Field),
		Relationships:  make(map[string]*Field),
		FlaggedFields:  make(map[string][]*Field),
	}

	// iterate through all fields
	for i := 0; i < modelType.NumField(); i++ {
		// get field
		field := modelType.Field(i)

		// get coal tag
		coalTag := field.Tag.Get("coal")

		// check for first field
		if i == 0 {
			// assert first field to be the base
			if field.Type != baseType {
				panic(`coal: expected an embedded "coal.Base" as the first struct field`)
			}

			// split tag
			baseTag := strings.Split(coalTag, ":")

			// check json tag
			if field.Tag.Get("json") != "-" {
				panic(`coal: expected to find a tag of the form 'json:"-"' on "coal.Base"`)
			}

			// check bson tag
			if field.Tag.Get("bson") != ",inline" {
				panic(`coal: expected to find a tag of the form 'bson:",inline"' on "coal.Base"`)
			}

			// check tag
			if len(baseTag) > 2 || baseTag[0] == "" {
				panic(`coal: expected to find a tag of the form 'coal:"plural-name[:collection]"' on "coal.Base"`)
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
		metaField := &Field{
			Name:      field.Name,
			Type:      field.Type,
			Kind:      fieldKind,
			JSONKey:   getJSONFieldName(&field),
			BSONField: getBSONFieldName(&field),
			Optional:  field.Type.Kind() == reflect.Ptr,
			index:     i,
		}

		// check if field is a valid to-one relationship
		if field.Type == toOneType || field.Type == optionalToOneType {
			if len(coalTags) > 0 && strings.Count(coalTags[0], ":") > 0 {
				// check tag
				if strings.Count(coalTags[0], ":") > 1 {
					panic(`coal: expected to find a tag of the form 'coal:"name:type"' on to-one relationship`)
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
				// check tag
				if strings.Count(coalTags[0], ":") > 1 {
					panic(`coal: expected to find a tag of the form 'coal:"name:type"' on to-many relationship`)
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
				panic(`coal: expected to find a tag of the form 'coal:"name:type:inverse"' on has-one relationship`)
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
				panic(`coal: expected to find a tag of the form 'coal:"name:type:inverse"' on has-many relationship`)
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

		// save additional tags as flags
		metaField.Flags = coalTags
		if metaField.Flags == nil {
			metaField.Flags = []string{}
		}

		// add field
		meta.Fields[metaField.Name] = metaField
		meta.OrderedFields = append(meta.OrderedFields, metaField)

		// add db fields
		if metaField.BSONField != "" {
			// check existence
			if meta.DatabaseFields[metaField.BSONField] != nil {
				panic(fmt.Sprintf(`coal: duplicate BSON field "%s"`, metaField.BSONField))
			}

			// add field
			meta.DatabaseFields[metaField.BSONField] = metaField
		}

		// add attributes
		if metaField.JSONKey != "" {
			// check existence
			if meta.Attributes[metaField.JSONKey] != nil {
				panic(fmt.Sprintf(`coal: duplicate JSON key "%s"`, metaField.JSONKey))
			}

			// add field
			meta.Attributes[metaField.JSONKey] = metaField
		}

		// add relationships
		if metaField.RelName != "" {
			// check existence
			if meta.Relationships[metaField.RelName] != nil {
				panic(fmt.Sprintf(`coal: duplicate relationship "%s"`, metaField.RelName))
			}

			// add field
			meta.Relationships[metaField.RelName] = metaField
		}

		// add flagged fields
		for _, flag := range metaField.Flags {
			// get list
			list, _ := meta.FlaggedFields[flag]

			// add field
			list = append(list, metaField)

			// save list
			meta.FlaggedFields[flag] = list
		}
	}

	// cache meta
	metaCache.Store(modelType, meta)

	return meta
}

// Make returns a pointer to a new zero initialized model e.g. *Post.
func (m *Meta) Make() Model {
	return reflect.New(m.Type).Interface().(Model)
}

// MakeSlice returns a pointer to a zero length slice of the model e.g. *[]*Post.
func (m *Meta) MakeSlice() interface{} {
	slice := reflect.MakeSlice(reflect.SliceOf(reflect.PtrTo(m.Type)), 0, 0)
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
	if len(values) > 0 && len(values[0]) > 0 {
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
	if len(values) > 0 && len(values[0]) > 0 {
		return values[0]
	}

	return strings.ToLower(field.Name)
}
