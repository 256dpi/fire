package fire

import (
	"reflect"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

// Map is a general purpose map.
type Map map[string]interface{}

/* internal */

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

func stringsToIDs(list []string) []bson.ObjectId {
	var ids []bson.ObjectId

	for _, str := range list {
		if bson.IsObjectIdHex(str) {
			ids = append(ids, bson.ObjectIdHex(str))
		}
	}

	return ids
}
