package coal

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/stick"
)

var metaMutex sync.Mutex
var metaCache = map[reflect.Type]*Meta{}

var itemMetaMutex sync.Mutex
var itemMetaCache = map[reflect.Type]*ItemMeta{}

var baseType = reflect.TypeOf(Base{})
var itemBaseType = reflect.TypeOf(ItemBase{})
var toOneType = reflect.TypeOf(ID{})
var optToOneType = reflect.TypeOf(&ID{})
var toManyType = reflect.TypeOf([]ID{})
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
	// The underlying item field.
	ItemField

	// The custom flags.
	Flags []string

	// The relationship status.
	ToOne   bool
	ToMany  bool
	HasOne  bool
	HasMany bool

	// The relationship information.
	RelName    string
	RelType    string
	RelInverse string
}

// ItemField contains the meta information about a single field of an item.
type ItemField struct {
	// The index of the field in the struct.
	Index int

	// The struct field name e.g. "TireSize".
	Name string

	// The struct field type and kind.
	Type reflect.Type
	Kind reflect.Kind

	// The JSON object key name e.g. "tire-size".
	JSONKey string

	// The BSON document key name e.g. "tire_size".
	BSONKey string

	// Whether the field is a pointer and thus optional.
	Optional bool

	// The item meta if field is a type embedding ItemBase.
	ItemMeta *ItemMeta
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

	// The request fields.
	RequestFields map[string]*Field

	// The flagged fields.
	FlaggedFields map[string][]*Field

	// The accessor.
	Accessor *stick.Accessor

	// The registered indexes.
	Indexes []Index
}

// ItemMeta stores extracted meta data from a model item.
type ItemMeta struct {
	// The struct type.
	Type reflect.Type

	// The struct type name e.g. "models.CarWheel".
	Name string

	// The struct fields.
	Fields map[string]*ItemField

	// The struct fields ordered.
	OrderedFields []*ItemField

	// The database fields.
	DatabaseFields map[string]*ItemField

	// The attributes.
	Attributes map[string]*ItemField

	// The accessor.
	Accessor *stick.Accessor
}

// GetMeta returns the meta structure for the specified model. It will always
// return the same value for the same model.
//
// Note: This method panics if the passed Model has invalid fields or tags.
func GetMeta(model Model) *Meta {
	// acquire mutex
	metaMutex.Lock()
	defer metaMutex.Unlock()

	// get type and name
	modelType := reflect.TypeOf(model).Elem()

	// check if meta has already been cached
	meta, ok := metaCache[modelType]
	if ok {
		return meta
	}

	// create new meta
	meta = &Meta{
		Type:           modelType,
		Name:           modelType.String(),
		Fields:         map[string]*Field{},
		DatabaseFields: map[string]*Field{},
		Attributes:     map[string]*Field{},
		Relationships:  map[string]*Field{},
		RequestFields:  map[string]*Field{},
		FlaggedFields:  map[string][]*Field{},
		Accessor:       stick.BuildAccessor(model, "Base"),
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
			if field.Type != baseType || !field.Anonymous {
				panic(`coal: expected an embedded "coal.Base" as the first struct field`)
			}

			// check json tag
			if field.Tag.Get("json") != "-" {
				panic(`coal: expected to find a tag of the form 'json:"-"' on "coal.Base"`)
			}

			// check bson tag
			if field.Tag.Get("bson") != ",inline" {
				panic(`coal: expected to find a tag of the form 'bson:",inline"' on "coal.Base"`)
			}

			// split tag
			baseTag := strings.Split(coalTag, ":")
			if len(baseTag) > 2 || baseTag[0] == "" {
				panic(`coal: expected to find a tag of the form 'coal:"plural-name[:collection]"' on "coal.Base"`)
			}

			// infer plural and collection names
			meta.PluralName = baseTag[0]
			meta.Collection = baseTag[0]
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

		// get field kind
		fieldKind := field.Type.Kind()
		if fieldKind == reflect.Ptr {
			fieldKind = field.Type.Elem().Kind()
		}

		// prepare field
		metaField := &Field{
			ItemField: ItemField{
				Index:    i,
				Name:     field.Name,
				Type:     field.Type,
				Kind:     fieldKind,
				JSONKey:  stick.JSON.GetKey(field),
				BSONKey:  stick.BSON.GetKey(field),
				Optional: field.Type.Kind() == reflect.Ptr,
				ItemMeta: GetItemMeta(field.Type),
			},
		}

		// check if field is a valid to-one relationship
		if field.Type == toOneType || field.Type == optToOneType {
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

		// add database fields
		if metaField.BSONKey != "" {
			// check existence
			if meta.DatabaseFields[metaField.BSONKey] != nil {
				panic(fmt.Sprintf(`coal: duplicate BSON key "%s"`, metaField.BSONKey))
			}

			// add field
			meta.DatabaseFields[metaField.BSONKey] = metaField
		}

		// add attributes
		if metaField.JSONKey != "" {
			// check existence
			if meta.Attributes[metaField.JSONKey] != nil {
				panic(fmt.Sprintf(`coal: duplicate JSON key "%s"`, metaField.JSONKey))
			}

			// add field
			meta.Attributes[metaField.JSONKey] = metaField
			meta.RequestFields[metaField.JSONKey] = metaField
		}

		// add relationships
		if metaField.RelName != "" {
			// check existence
			if meta.Relationships[metaField.RelName] != nil {
				panic(fmt.Sprintf(`coal: duplicate relationship "%s"`, metaField.RelName))
			}

			// add field
			meta.Relationships[metaField.RelName] = metaField
			meta.RequestFields[metaField.RelName] = metaField
		}

		// add flagged fields
		for _, flag := range metaField.Flags {
			// get list
			list := meta.FlaggedFields[flag]

			// add field
			list = append(list, metaField)

			// save list
			meta.FlaggedFields[flag] = list
		}
	}

	// add tag wildcard index
	meta.Indexes = append(meta.Indexes, Index{
		Keys: bson.D{
			{Key: "_tg.$**", Value: 1},
		},
	})

	// cache meta
	metaCache[modelType] = meta

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

// GetItemMeta returns the meta structure for the specified item type. It will
// always return the same value for the same item.
func GetItemMeta(typ reflect.Type) *ItemMeta {
	// check if meta has already been cached
	itemMetaMutex.Lock()
	meta, ok := itemMetaCache[typ]
	itemMetaMutex.Unlock()
	if ok {
		return meta
	}

	// unwrap pointer
	for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}

	// check if embedding item
	if typ.NumField() == 0 || typ.Field(0).Type != itemBaseType || !typ.Field(0).Anonymous {
		return nil
	}

	// TODO: Validate json and bson tags.

	// prepare meta
	meta = &ItemMeta{
		Type:           typ,
		Name:           typ.String(),
		Fields:         map[string]*ItemField{},
		DatabaseFields: map[string]*ItemField{},
		Attributes:     map[string]*ItemField{},
		Accessor:       stick.BuildAccessor(reflect.New(typ).Interface(), "ItemBase"),
	}

	// parse fields
	for i := 1; i < typ.NumField(); i++ {
		// get field
		field := typ.Field(i)

		// get field kind
		fieldKind := field.Type.Kind()
		if fieldKind == reflect.Ptr {
			fieldKind = field.Type.Elem().Kind()
		}

		// prepare meta
		metaField := &ItemField{
			Index:    i,
			Name:     field.Name,
			Type:     field.Type,
			Kind:     fieldKind,
			JSONKey:  stick.JSON.GetKey(field),
			BSONKey:  stick.BSON.GetKey(field),
			Optional: field.Type.Kind() == reflect.Ptr,
			ItemMeta: GetItemMeta(field.Type),
		}

		// add field
		meta.Fields[metaField.Name] = metaField
		meta.OrderedFields = append(meta.OrderedFields, metaField)

		// add database fields
		if metaField.BSONKey != "" {
			// check existence
			if meta.DatabaseFields[metaField.BSONKey] != nil {
				panic(fmt.Sprintf(`coal: duplicate BSON key "%s"`, metaField.BSONKey))
			}

			// add field
			meta.DatabaseFields[metaField.BSONKey] = metaField
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
	}

	// cache meta
	itemMetaMutex.Lock()
	itemMetaCache[typ] = meta
	itemMetaMutex.Unlock()

	return meta
}
