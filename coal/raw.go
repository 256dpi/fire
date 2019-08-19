package coal

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

// Raw represents a raw BSON object that also supports JSON coding.
type Raw bson.Raw

// MustRaw will encode the specified object as a raw value and panic if there
// was an error.
func MustRaw(obj map[string]interface{}) Raw {
	var raw Raw
	err := raw.Set(obj)
	if err != nil {
		panic(err)
	}
	return raw
}

// Get will return the object.
func (r *Raw) Get() (map[string]interface{}, error) {
	// decode BSON
	var obj map[string]interface{}
	err := bson.Unmarshal(*r, &obj)
	return obj, err
}

// Set will set the object.
func (r *Raw) Set(obj map[string]interface{}) error {
	// encode BSON
	res, err := bson.Marshal(obj)
	if err != nil {
		return err
	}

	// set result
	*r = res

	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (r *Raw) MarshalJSON() ([]byte, error) {
	// get object
	obj, err := r.Get()
	if err != nil {
		return nil, err
	}

	// encode JSON
	return json.Marshal(obj)
}

// UnmarshalJSON implements the json.Marshaler interface.
func (r *Raw) UnmarshalJSON(data []byte) error {
	// decode JSON
	var obj map[string]interface{}
	err := json.Unmarshal(data, &obj)
	if err != nil {
		return err
	}

	// set object
	return r.Set(obj)
}
