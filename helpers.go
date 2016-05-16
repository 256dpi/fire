package fire

import (
	"reflect"
	"strings"

	"github.com/manyminds/api2go"
	"gopkg.in/mgo.v2/bson"
)

// query helper functions

func getQueryParam(req *api2go.Request, param string) (interface{}, bool) {
	if len(req.QueryParams[param]) == 0 {
		return "", false
	}

	if !strings.HasSuffix(param, "-id") && !strings.HasSuffix(param, "ID") {
		return req.QueryParams[param][0], true
	}

	if !bson.IsObjectIdHex(req.QueryParams[param][0]) {
		return "", false
	}

	return bson.ObjectIdHex(req.QueryParams[param][0]), true
}

// reflect helper functions

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
