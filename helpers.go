package fire

import (
	"reflect"
	"strings"
)

func getJSONFieldName(field *reflect.StructField) string {
	tag := field.Tag.Get("json")
	values := strings.Split(tag, ",")

	// check for "-"
	if tag == "-" {
		return ""
	}

	// check first value
	if len(tag) > 0 || len(values[0]) > 0 {
		return values[0]
	}

	return field.Name
}

func getBSONFieldName(field *reflect.StructField) string {
	tag := field.Tag.Get("bson")
	values := strings.Split(tag, ",")

	// check for "-"
	if tag == "-" {
		return ""
	}

	// check first value
	if len(tag) > 0 || len(values[0]) > 0 {
		return values[0]
	}

	return strings.ToLower(field.Name)
}

func stringInList(str string, list []string) bool {
	for _, val := range list {
		if val == str {
			return true
		}
	}

	return false
}
