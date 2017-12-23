package coal

import "gopkg.in/mgo.v2/bson"

// C is a short-hand function to extract the collection of a model.
func C(m Model) string {
	return Init(m).Meta().Collection
}

// F is a short-hand function to extract the BSON field name of a model
// attribute.
func F(m Model, field string) string {
	return Init(m).Meta().MustFindField(field).BSONName
}

// A is a short-hand function to extract the JSON attribute name of a model
// attribute.
func A(m Model, field string) string {
	return Init(m).Meta().MustFindField(field).JSONName
}

// R is a short-hand function to extract the relationship name of a model
// attribute.
func R(m Model, field string) string {
	return Init(m).Meta().MustFindField(field).RelName
}

// P is a short-hand function to get a pointer of the passed id.
func P(id bson.ObjectId) *bson.ObjectId {
	return &id
}

// Unique is a helper to get a unique list of object ids.
func Unique(ids []bson.ObjectId) []bson.ObjectId {
	// prepare map
	m := make(map[bson.ObjectId]bool)
	l := make([]bson.ObjectId, 0, len(ids))

	for _, id := range ids {
		if _, ok := m[id]; !ok {
			m[id] = true
			l = append(l, id)
		}
	}

	return l
}
