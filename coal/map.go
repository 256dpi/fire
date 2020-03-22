package coal

// Map represents a simple map. It provides methods to marshal and unmarshal
// concrete types to and from the map using JSON or BSON coding.
type Map map[string]interface{}

// MustMap will marshal the specified object to a map and panic on any error.
func MustMap(from interface{}, coding Coding) Map {
	var m Map
	m.MustMarshal(from, coding)
	return m
}

// Unmarshal will unmarshal the map to the specified value. If the value already
// contains data, only fields present in the map are overwritten.
func (m *Map) Unmarshal(to interface{}, coding Coding) error {
	return coding.Transfer(*m, to)
}

// MustUnmarshal will unmarshal and panic on error.
func (m *Map) MustUnmarshal(to interface{}, coding Coding) {
	err := m.Unmarshal(to, coding)
	if err != nil {
		panic(err)
	}
}

// Marshal will marshal the specified value to the map. If the map already
// contains data, only fields present in the value are overwritten.
func (m *Map) Marshal(from interface{}, coding Coding) error {
	return coding.Transfer(from, m)
}

// MustMarshal will marshal and panic on error.
func (m *Map) MustMarshal(from interface{}, coding Coding) {
	err := m.Marshal(from, coding)
	if err != nil {
		panic(err)
	}
}
