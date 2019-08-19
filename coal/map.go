package coal

import (
	"go.mongodb.org/mongo-driver/bson"
)

// Map represents a simple BSON map. In contrast to bson.M it provides methods
// to marshal and unmarshal concrete types to and from the map.
type Map map[string]interface{}

// MustMap will marshal the specified object to a map and panic on any error.
func MustMap(from interface{}) Map {
	var m Map
	m.MustMarshal(from)
	return m
}

// Unmarshal will unmarshal the raw value to the type pointed by val.
func (m *Map) Unmarshal(to interface{}) error {
	// marshal to BSON
	bytes, err := bson.Marshal(*m)
	if err != nil {
		return err
	}

	// unmarshal BSON
	return bson.Unmarshal(bytes, to)
}

// MustUnmarshal will unmarshal and panic on error.
func (m *Map) MustUnmarshal(to interface{}) {
	// unmarshal and panic on error
	err := m.Unmarshal(to)
	if err != nil {
		panic(err)
	}
}

// Marshal will encode the type pointed by val and store the result.
func (m *Map) Marshal(from interface{}) error {
	// marshal to BSON
	bytes, err := bson.Marshal(from)
	if err != nil {
		return err
	}

	// reset
	*m = nil

	// unmarshal BSON
	return bson.Unmarshal(bytes, m)
}

// MustMarshal will marshal and panic on error.
func (m *Map) MustMarshal(from interface{}) {
	// marshal and panic on error
	err := m.Marshal(from)
	if err != nil {
		panic(err)
	}
}
