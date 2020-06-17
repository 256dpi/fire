package stick

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"
)

// Coding defines an encoding, decoding and transfer scheme.
type Coding string

// The available coding schemes.
const (
	JSON Coding = "json"
	BSON Coding = "bson"
)

// Marshal will encode the specified value into a byte sequence.
func (c Coding) Marshal(in interface{}) ([]byte, error) {
	switch c {
	case JSON:
		buf, err := json.Marshal(in)
		return buf, xo.W(err)
	case BSON:
		buf, err := bson.Marshal(in)
		return buf, xo.W(err)
	default:
		panic(fmt.Sprintf("coal: unknown coding %q", c))
	}
}

// Unmarshal will decode the specified value from the provided byte sequence.
func (c Coding) Unmarshal(in []byte, out interface{}) error {
	switch c {
	case JSON:
		return xo.W(json.Unmarshal(in, out))
	case BSON:
		return xo.W(bson.Unmarshal(in, out))
	default:
		panic(fmt.Sprintf("coal: unknown coding %q", c))
	}
}

// SafeUnmarshal will decode the specified value from the provided byte sequence.
// It will preserve JSON numbers when decoded into an interface{} value.
func (c Coding) SafeUnmarshal(in []byte, out interface{}) error {
	switch c {
	case JSON:
		dec := json.NewDecoder(bytes.NewReader(in))
		dec.UseNumber()
		return xo.W(dec.Decode(out))
	case BSON:
		return xo.W(bson.Unmarshal(in, out))
	default:
		panic(fmt.Sprintf("coal: unknown coding %q", c))
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

// GetJSONKey will return the JSON key for the specified struct field.
func GetJSONKey(field *reflect.StructField) string {
	// get tag
	tag := field.Tag.Get("json")

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

	return field.Name
}

// GetBSONKey will return the BSON key for the specified struct field.
func GetBSONKey(field *reflect.StructField) string {
	// get tag
	tag := field.Tag.Get("bson")

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

	return strings.ToLower(field.Name)
}
