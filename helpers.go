package fire

import (
	"reflect"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

// M is a general purpose map.
type M map[string]interface{}

// StringInList returns whether the supplied strings can be found in the list.
func StringInList(list []string, str string) bool {
	for _, val := range list {
		if val == str {
			return true
		}
	}

	return false
}

/* internal */

func newSlicePointer(from interface{}) interface{} {
	slice := reflect.MakeSlice(reflect.SliceOf(reflect.TypeOf(from)), 0, 0)
	pointer := reflect.New(slice.Type())
	pointer.Elem().Set(slice)
	return pointer.Interface()
}

func newStructPointer(from interface{}) interface{} {
	return reflect.New(reflect.TypeOf(from).Elem()).Interface()
}

func sliceContent(pointer interface{}) interface{} {
	return reflect.ValueOf(pointer).Elem().Interface()
}

func getJSONFieldName(field *reflect.StructField) string {
	tag := field.Tag.Get("json")
	values := strings.Split(tag, ",")

	// check first value
	if len(tag) > 0 || len(values[0]) > 0 {
		return values[0]
	}

	return field.Name
}

func getBSONFieldName(field *reflect.StructField) string {
	tag := field.Tag.Get("bson")
	values := strings.Split(tag, ",")

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
