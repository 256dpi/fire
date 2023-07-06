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
// a custom key instead of their position in the list.
//
// When using with custom types the type should implement the following methods:
//
//	func (l *Links) UnmarshalJSON(bytes []byte) error {
//		return stick.UnmarshalKeyedList(stick.JSON, bytes, l, func(link Link) string {
//			return link.Ref
//		})
//	}
//
//	func (l *Links) UnmarshalBSONValue(typ bsontype.Type, bytes []byte) error {
//		return stick.UnmarshalKeyedList(stick.BSON, stick.InternalBSONValue(typ, bytes), l, func(link Link) string {
//			return link.Ref
//		})
//	}
func UnmarshalKeyedList[T any, L ~[]T, K comparable](c Coding, data []byte, list *L, mapper func(T) K) error {
	// prepare index
	index := map[K]T{}
	for i := 0; i < len(*list); i++ {
		item := (*list)[i]
		key := mapper(item)
		index[key] = item
	}

	// unmarshal into typed empty slice
	var temp []T
	err := c.Unmarshal(data, &temp)
	if err != nil {
		return err
	}

	// unmarshal into empty slice of maps
	var rawList []Map
	err = c.SafeUnmarshal(data, &rawList)
	if err != nil {
		return err
	}

	// handle nil
	if temp == nil {
		*list = nil
		return nil
	}

	// create new list
	newList := make([]T, 0, len(temp))

	// merge links
	for i := range temp {
		// get existing item
		extItem, ok := index[mapper(temp[i])]

		// directly add new item if missing
		if !ok {
			newList = append(newList, temp[i])
			continue
		}

		// transfer from raw item to existing item
		err = c.Transfer(rawList[i], &extItem)
		if err != nil {
			return err
		}

		// add existing item
		newList = append(newList, extItem)
	}

	// set list
	*list = newList

	return nil
}
