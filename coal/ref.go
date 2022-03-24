package coal

import (
	"go.mongodb.org/mongo-driver/bson"
)

var maxID = ID{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}

// Ref is a dynamic reference to a document in a collection.
type Ref struct {
	// The referenced document collection.
	Coll string `bson:"coll"`

	// The referenced document id.
	ID ID `bson:"id"`
}

// R is shorthand to retrieve a reference to the provided model.
func R(model Model) Ref {
	return Ref{
		Coll: GetMeta(model).Collection,
		ID:   model.ID(),
	}
}

// IsZero returns whether the reference is zero.
func (r *Ref) IsZero() bool {
	return r.Coll == "" && r.ID.IsZero()
}

// IsValid returns whether the reference is valid.
func (r *Ref) IsValid() bool {
	return r.Coll != "" && !r.ID.IsZero()
}

// AnyRef will return a query partial that can be used to query all references
// to a specific collection.
func AnyRef(model Model) bson.M {
	// get collection
	coll := GetMeta(model).Collection

	return bson.M{
		"$gte": Ref{Coll: coll, ID: ID{}},
		"$lte": Ref{Coll: coll, ID: maxID},
	}
}
