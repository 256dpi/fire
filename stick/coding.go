package stick

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

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
		return json.Marshal(in)
	case BSON:
		return bson.Marshal(in)
	default:
		panic(fmt.Sprintf("coal: unknown coding %q", c))
	}
}

// Unmarshal will decode the specified value from the provided byte sequence.
func (c Coding) Unmarshal(in []byte, out interface{}) error {
	switch c {
	case JSON:
		return json.Unmarshal(in, out)
	case BSON:
		return bson.Unmarshal(in, out)
	default:
		panic(fmt.Sprintf("coal: unknown coding %q", c))
	}
}

// Transfer will transfer data from one value to another using.
func (c Coding) Transfer(in, out interface{}) error {
	// marshal
	bytes, err := c.Marshal(in)
	if err != nil {
		return err
	}

	// unmarshal
	err = c.Unmarshal(bytes, out)
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
