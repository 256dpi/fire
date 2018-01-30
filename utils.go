package fire

import (
	"errors"

	"gopkg.in/mgo.v2/bson"
)

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
