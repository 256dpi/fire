package fire

import (
	"errors"
	"strings"

	"gopkg.in/mgo.v2/bson"
)

func stringInList(str string, list []string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}

	return false
}

func toObjectIDList(list []string) ([]bson.ObjectId, error) {
	var ids []bson.ObjectId

	for _, str := range list {
		if !bson.IsObjectIdHex(str) {
			return nil, errors.New("invalid id")
		}

		ids = append(ids, bson.ObjectIdHex(str))
	}

	return ids, nil
}

func operationsAsStrings(ops []Operation) []string {
	list := make([]string, len(ops))
	for i, op := range ops {
		list[i] = op.String()
	}
	return list
}

func joinOperations(ops []Operation, sep string) string {
	return strings.Join(operationsAsStrings(ops), sep)
}
