package stick

import "reflect"

// Unwrap will unwrap pointers and return the underlying value.
func Unwrap(v interface{}) interface{} {
	// get value
	val := reflect.ValueOf(v)

	// unwrap pointers
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	return val.Interface()
}
