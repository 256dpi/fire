package coal

import (
	"reflect"
	"strings"

	"github.com/256dpi/lungo/bsonkit"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var unsafeOperators = map[string]bool{
	// query
	"$expr":       true,
	"$jsonSchema": true,
	"$where":      true,
	"$elemMatch":  true,

	// update
	"$rename": true,
}

var systemFields = map[string]bool{
	"_id": true,
	"_lk": true,
	"_tk": true,
	"_tg": true,
}

var systemFieldPrefixes = []string{
	"_tg.",
}

// Translator is capable of translating filter, update and sort documents from
// struct field names to database fields names. It also checks documents against
// as list of unsafe operators. Field names may be prefixed with a "#" to bypass
// any validation.
type Translator struct {
	meta *Meta
}

// NewTranslator will return a translator for the specified model.
func NewTranslator(model Model) *Translator {
	return &Translator{
		meta: GetMeta(model),
	}
}

// Field will translate the specified field. The field may be a path to a nested
// item field or begin wih a "#" (after prefix) to specify a unknown field.
func (t *Translator) Field(field string) (string, error) {
	err := t.field(&field)
	if err != nil {
		return "", err
	}

	return field, nil
}

// Document will convert the provided filter or update document and translate
// all field names to refer to known database fields. It will also validate the
// filter or update and return an error for unsafe expressions or operators.
func (t *Translator) Document(query bson.M) (bson.D, error) {
	// convert
	doc, err := t.convert(query)
	if err != nil {
		return nil, err
	}

	// translate
	err = t.value(doc, false)
	if err != nil {
		return nil, err
	}

	return doc, err
}

// Sort will convert the provided sort array to a sort document and translate
// all field names to refer to known database fields.
func (t *Translator) Sort(fields []string) (bson.D, error) {
	// convert
	doc := Sort(fields...)

	// translate
	err := t.value(doc, false)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (t *Translator) value(value interface{}, skipTranslation bool) error {
	// translate document
	if doc, ok := value.(bson.D); ok {
		for i, pair := range doc {
			// check if operator
			if strings.HasPrefix(pair.Key, "$") {
				// validate operator
				if unsafeOperators[pair.Key] {
					return xo.F("unsafe operator %q", pair.Key)
				}
			} else if !skipTranslation {
				// translate field
				err := t.field(&doc[i].Key)
				if err != nil {
					return err
				}
			}
		}
	}

	// check value
	switch value := value.(type) {
	case bson.A:
		for _, item := range value {
			err := t.value(item, skipTranslation)
			if err != nil {
				return err
			}
		}
		return nil
	case bson.D:
		for _, item := range value {
			err := t.value(item.Value, skipTranslation || !strings.HasPrefix(item.Key, "$"))
			if err != nil {
				return err
			}
		}
		return nil
	case nil, int32, int64, float64, string, bool, primitive.Null,
		primitive.ObjectID, primitive.DateTime, primitive.Timestamp,
		primitive.Regex, primitive.Binary, primitive.Decimal128:
		return nil
	default:
		return xo.F("unsupported type %T", value)
	}
}

func (t *Translator) field(path *string) error {
	// handle raw paths
	if strings.HasPrefix(*path, "#") {
		*path = (*path)[1:]
		return nil
	}

	// get first field
	field := bsonkit.PathSegment(*path)

	// check if known
	if t.meta.DatabaseFields[field] != nil {
		return nil
	}

	// check if system
	if systemFields[field] {
		return nil
	}
	for _, prefix := range systemFieldPrefixes {
		if strings.HasPrefix(field, prefix) {
			return nil
		}
	}

	// check meta
	structField := t.meta.Fields[field]
	if structField == nil {
		return xo.F("unknown field %q", field)
	} else if structField.BSONKey == "" {
		return xo.F("virtual field %q", field)
	}

	// replace single field path
	if field == *path {
		*path = structField.BSONKey
		return nil
	}

	/* handle multi field path */

	// TODO: Use bsonkit.PathBuilder?

	// split path
	fields := strings.Split(*path, ".")

	// replace first fields
	fields[0] = structField.BSONKey

	// handle other fields
	meta := &structField.ItemField
	for i, field := range fields[1:] {
		// handle slice index
		_, ok := bsonkit.ParseIndex(field)
		if ok && meta.Kind == reflect.Slice {
			continue
		}

		// check meta
		if meta == nil || meta.ItemMeta == nil {
			return xo.F("unknown field %q", *path)
		}

		// check field
		itemField := meta.ItemMeta.Fields[field]
		if itemField == nil {
			return xo.F("unknown field %q", *path)
		} else if itemField.BSONKey == "" {
			return xo.F("virtual field %q", *path)
		}

		// replace field
		fields[i+1] = itemField.BSONKey

		// advance meta
		meta = itemField
	}

	// replace path
	*path = strings.Join(fields, ".")

	return nil
}

func (t *Translator) convert(in bson.M) (bson.D, error) {
	// attempt fast conversion
	doc, err := bsonkit.Convert(in)
	if err == nil {
		return *doc, nil
	}

	// otherwise, convert safely
	doc, err = bsonkit.Transform(in)
	if err != nil {
		return nil, xo.W(err)
	}

	return *doc, nil
}
