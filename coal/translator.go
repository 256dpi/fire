package coal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/256dpi/lungo/bsonkit"
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
}

// Translator is capable of translating query, update and sort documents from
// struct field names to database fields names.
type Translator struct {
	meta *Meta
}

// Translate will return a translator for the specified model.
func Translate(model Model) *Translator {
	return &Translator{
		meta: GetMeta(model),
	}
}

// Document will convert the provided query or update document and translate
// all field names to refer to known database fields. It will also validate the
// query or update and return an error if an unsafe operator is used.
func (t *Translator) Document(queryOrUpdate bson.M) (bson.D, error) {
	// convert
	doc, err := t.convert(queryOrUpdate)
	if err != nil {
		return nil, err
	}

	// translate
	err = t.value(doc)
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
	err := t.value(doc)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (t *Translator) value(value interface{}) error {
	// translate document
	if doc, ok := value.(bson.D); ok {
		for i, pair := range doc {
			// check if operator
			if strings.HasPrefix(pair.Key, "$") {
				// validate operator
				if unsafeOperators[pair.Key] {
					return fmt.Errorf("unsafe operator %q", pair.Key)
				}
			} else {
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
			err := t.value(item)
			if err != nil {
				return err
			}
		}
		return nil
	case bson.D:
		for _, item := range value {
			err := t.value(item.Value)
			if err != nil {
				return err
			}
		}
		return nil
	case nil, int32, int64, float64, string, bool, primitive.Null,
		primitive.ObjectID, primitive.DateTime, primitive.Timestamp,
		primitive.Regex, primitive.Binary:
		return nil
	default:
		return fmt.Errorf("unsupported type %T", value)
	}
}

func (t *Translator) field(field *string) error {
	// check if known
	if t.meta.DatabaseFields[*field] != nil {
		return nil
	}

	// check if system
	if systemFields[*field] {
		return nil
	}

	// check meta
	structField := t.meta.Fields[*field]
	if structField == nil {
		return fmt.Errorf("unknown field %q", *field)
	} else if structField.BSONField == "" {
		return fmt.Errorf("virtual field %q", *field)
	}

	// replace field
	*field = structField.BSONField

	return nil
}

func (t *Translator) convert(in bson.M) (out bson.D, err error) {
	// catch panic due to unsupported type
	defer func() {
		str := recover()
		if str != nil {
			err = errors.New(str.(string)[9:])
		}
	}()

	// TODO: Use bsonkit.Transform?

	// convert
	out = *bsonkit.Convert(in)

	return
}
