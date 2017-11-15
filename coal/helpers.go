package coal

import "gopkg.in/mgo.v2/bson"

// Optional returns a pointer to the passed id.
func Optional(id bson.ObjectId) *bson.ObjectId {
	return &id
}
