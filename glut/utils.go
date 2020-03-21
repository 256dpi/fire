package glut

import "fmt"

// GetKey will get the key of a value.
func GetKey(value Value) (string, error) {
	// get meta
	meta := GetMeta(value)

	// prepare key
	key := meta.Key

	// check if extended
	extendedValue, ok := value.(ExtendedValue)
	if !ok {
		return key, nil
	}

	// get extension
	extension, err := extendedValue.GetExtension()
	if err != nil {
		return "", err
	}

	// check extension
	if extension == "" {
		return "", fmt.Errorf("missing extension")
	}

	return key + extension, nil
}
