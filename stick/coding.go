package stick

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Coding defines an encoding, decoding and transfer scheme.
type Coding string

// The available coding schemes.
const (
	JSON Coding = "json"
	BSON Coding = "bson"
)

var bsonMagic = []byte("STICK")

// InternalBSONValue will return a byte sequence for an internal BSON value.
func InternalBSONValue(typ bsontype.Type, bytes []byte) []byte {
	buf := make([]byte, 0, len(bsonMagic)+1+len(bytes))
	buf = append(buf, bsonMagic...)
	buf = append(buf, byte(typ))
	return append(buf, bytes...)
}

// Marshal will encode the specified value into a byte sequence.
//
// Note: When marshalling a non document compatible type to BSON the result is a
// custom byte sequence that can only be unmarshalled by this codec.
func (c Coding) Marshal(in interface{}) ([]byte, error) {
	switch c {
	case JSON:
		buf, err := json.Marshal(in)
		return buf, xo.W(err)
	case BSON:
		// replace nil with null
		if in == nil {
			in = primitive.Null{}
		}

		// encode as document
		buf, err := bson.Marshal(in)
		if err == nil {
			return buf, nil
		}

		// otherwise encode as internal value
		typ, buf, err := bson.MarshalValue(in)
		if err == nil {
			return InternalBSONValue(typ, buf), nil
		}

		return nil, xo.W(err)
	default:
		panic(fmt.Sprintf("stick: unknown coding %q", c))
	}
}

// Unmarshal will decode the specified value from the provided byte sequence.
func (c Coding) Unmarshal(in []byte, out interface{}) error {
	switch c {
	case JSON:
		return xo.W(json.Unmarshal(in, out))
	case BSON:
		// check if internal value
		if bytes.HasPrefix(in, bsonMagic) {
			raw := bson.RawValue{Value: in[len(bsonMagic)+1:], Type: bsontype.Type(in[len(bsonMagic)])}
			err := raw.Unmarshal(out)
			return xo.W(err)
		}

		return xo.W(bson.Unmarshal(in, out))
	default:
		panic(fmt.Sprintf("stick: unknown coding %q", c))
	}
}

// SafeUnmarshal will decode the specified value from the provided byte sequence.
// It will preserve JSON numbers when decoded into an interface{} value.
func (c Coding) SafeUnmarshal(in []byte, out interface{}) error {
	switch c {
	case JSON:
		// use number mode
		dec := json.NewDecoder(bytes.NewReader(in))
		dec.UseNumber()
		return xo.W(dec.Decode(out))
	default:
		return c.Unmarshal(in, out)
	}
}

// Transfer will transfer data from one value to another using.
func (c Coding) Transfer(in, out interface{}) error {
	// marshal
	data, err := c.Marshal(in)
	if err != nil {
		return err
	}

	// unmarshal
	err = c.Unmarshal(data, out)
	if err != nil {
		return err
	}

	return nil
}

// MimeType returns the coding mim type.
func (c Coding) MimeType() string {
	switch c {
	case JSON:
		return "application/json"
	case BSON:
		return "application/bson"
	default:
		panic(fmt.Sprintf("stick: unknown coding %q", c))
	}
}

// GetKey will return the coding key for the specified struct field.
func (c Coding) GetKey(field reflect.StructField) string {
	// get tag
	tag := field.Tag.Get(string(c))

	// check for "-"
	if tag == "-" {
		return ""
	}

	// split
	values := strings.Split(tag, ",")

	// check first value
	if len(values) > 0 && len(values[0]) > 0 {
		return values[0]
	}

	// prepare name
	name := field.Name
	if c == BSON {
		name = strings.ToLower(name)
	}

	return name
}

// UnmarshalKeyedList will unmarshal a list and match objects by comparing
// the key in the specified field instead of their position in the list.
//
// When using with custom types the type should implement the following methods:
//
// 	func (l *Links) UnmarshalBSONValue(typ bsontype.Type, bytes []byte) error {
// 		return stick.BSON.UnmarshalKeyedList(stick.InternalBSONValue(typ, bytes), l, "Ref")
// 	}
//
// 	func (l *Links) UnmarshalJSON(bytes []byte) error {
// 		return stick.JSON.UnmarshalKeyedList(bytes, l, "Ref")
// 	}
//
func (c Coding) UnmarshalKeyedList(data []byte, list interface{}, field string) error {
	// get existing list
	existingList := reflect.ValueOf(list).Elem()

	// get item type
	itemType := existingList.Type().Elem()

	// create index
	indexType := reflect.MapOf(reflect.TypeOf(""), itemType)
	index := reflect.MakeMapWithSize(indexType, existingList.Len())

	// determine coding key
	codingKey := field
	if itemType.Kind() == reflect.Struct {
		structField, ok := itemType.FieldByName(field)
		if ok {
			codingKey = c.GetKey(structField)
		}
	}

	// fill index
	for i := 0; i < existingList.Len(); i++ {
		// get item
		item := existingList.Index(i)

		// get key
		var key string
		if item.Type().Kind() == reflect.Map {
			key = item.MapIndex(reflect.ValueOf(codingKey)).Interface().(string)
		} else {
			key = item.FieldByName(field).Interface().(string)
		}

		// store item
		index.SetMapIndex(reflect.ValueOf(key), item)
	}

	// unmarshal into dynamic slice
	var temp []map[string]interface{}
	err := c.SafeUnmarshal(data, &temp)
	if err != nil {
		return err
	}

	// check nil
	if temp == nil {
		reflect.ValueOf(list).Elem().Set(reflect.Zero(existingList.Type()))
		return nil
	}

	// create new list
	newList := reflect.MakeSlice(existingList.Type(), 0, len(temp))

	// merge links
	for _, obj := range temp {
		// match existing item
		item := index.MapIndex(reflect.ValueOf(obj[codingKey].(string)))

		// ensure item if not found
		if !item.IsValid() {
			if itemType.Kind() == reflect.Map {
				item = reflect.MakeMap(itemType)
			} else {
				item = reflect.Zero(itemType)
			}
		}

		// transfer to item
		pointer := reflect.New(item.Type())
		pointer.Elem().Set(item)
		err = c.Transfer(obj, pointer.Interface())
		if err != nil {
			return err
		}

		// add item
		newList = reflect.Append(newList, pointer.Elem())
	}

	// set list
	reflect.ValueOf(list).Elem().Set(newList)

	return nil
}
