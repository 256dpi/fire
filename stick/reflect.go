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

// IsNil returns whether the provided value is nil while correctly handling typed
// nils.
func IsNil(v interface{}) bool {
	// check plain nil
	if v == nil {
		return true
	}

	// check typed nils
	value := reflect.ValueOf(v)
	switch value.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		return value.IsNil()
	}

	return false
}
