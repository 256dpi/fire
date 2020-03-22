package stick

import (
	"encoding/json"
	"fmt"

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
