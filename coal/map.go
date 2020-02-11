package coal

import (
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson"
)

// Map represents a simple map. In contrast to bson.M it provides methods
// to marshal and unmarshal concrete types to and from the map using various
// transfer methods.
type Map map[string]interface{}

// MustMap will marshal the specified object to a map and panic on any error.
func MustMap(from interface{}, transfer Transfer) Map {
	var m Map
	m.MustMarshal(from, transfer)
	return m
}

// Unmarshal will unmarshal the map to the specified value. If the value already
// contains data, only fields present in the map are overwritten.
func (m *Map) Unmarshal(to interface{}, transfer Transfer) error {
	return transfer(*m, to)
}

// MustUnmarshal will unmarshal and panic on error.
func (m *Map) MustUnmarshal(to interface{}, transfer Transfer) {
	MustTransfer(*m, to, transfer)
}

// Marshal will marshal the specified value to the map. If the map already
// contains data, only fields present in the value are overwritten.
func (m *Map) Marshal(from interface{}, transfer Transfer) error {
	return transfer(from, m)
}

// MustMarshal will marshal and panic on error.
func (m *Map) MustMarshal(from interface{}, transfer Transfer) {
	MustTransfer(from, m, transfer)
}

// Transfer is a generic transfer function.
type Transfer func(in, out interface{}) error

// TransferBSON will transfer BSON data from one value to another.
func TransferBSON(in, out interface{}) error {
	// marshal
	bytes, err := bson.Marshal(in)
	if err != nil {
		return err
	}

	// unmarshal
	err = bson.Unmarshal(bytes, out)
	if err != nil {
		return err
	}

	return nil
}

// TransferJSON will transfer JSON data from one value to another.
func TransferJSON(in, out interface{}) error {
	// marshal
	bytes, err := json.Marshal(in)
	if err != nil {
		return err
	}

	// unmarshal
	err = json.Unmarshal(bytes, out)
	if err != nil {
		return err
	}

	return nil
}

// MustTransfer will call the provided transfer function and panic on errors.
func MustTransfer(in, out interface{}, transfer Transfer) {
	err := transfer(in, out)
	if err != nil {
		panic(err)
	}
}
