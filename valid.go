package fire

import (
	"github.com/asaskevich/govalidator"
	"gopkg.in/mgo.v2/bson"
)

func init() {
	// register the custom object-id validator
	govalidator.CustomTypeTagMap.Set("object-id", func(i interface{}, o interface{}) bool {
		// check object
		if id, ok := i.(bson.ObjectId); ok {
			return id.Valid()
		}

		// check pointer
		if id, ok := i.(*bson.ObjectId); ok {
			if id != nil {
				return id.Valid()
			}

			return true
		}

		// check slice
		if ids, ok := i.([]bson.ObjectId); ok {
			for _, id := range ids {
				if !id.Valid() {
					return false
				}
			}
			return true
		}

		panic("coal: unsupported field for object-id validator")
	})
}
