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

// GetInt will attempt to get an int64 from the provided value.
func GetInt(v interface{}) (int64, bool) {
	switch v := v.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	default:
		return 0, false
	}
}

// GetUint will attempt to get an uint64 from the provided value.
func GetUint(v interface{}) (uint64, bool) {
	switch v := v.(type) {
	case uint:
		return uint64(v), true
	case uint8:
		return uint64(v), true
	case uint16:
		return uint64(v), true
	case uint32:
		return uint64(v), true
	case uint64:
		return v, true
	default:
		return 0, false
	}
}

// GetFloat will attempt to get a float from the provided value.
func GetFloat(v interface{}) (float64, bool) {
	switch v := v.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	default:
		return 0, false
	}
}
