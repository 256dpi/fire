package stick

import "reflect"

// GetType will get the underlying reflect type for the specified value. It
// will unwrap pointers automatically.
func GetType(v interface{}) reflect.Type {
	// get type
	typ := reflect.TypeOf(v)

	// unwrap pointer
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return typ
}
