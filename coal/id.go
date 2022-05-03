package coal

import (
	"time"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ID is shorthand type for the object id.
type ID = primitive.ObjectID

// Z is a shorthand to get a zero object id.
func Z() ID {
	return ID{}
}

// P is a shorthand function to get a pointer of the specified object id.
func P(id ID) *ID {
	return &id
}

// N is a shorthand function to get a typed nil object id pointer.
func N() *ID {
	return nil
}

// New will return a new object id, optionally using a custom timestamp.
func New(timestamp ...time.Time) ID {
	// check timestamp
	if len(timestamp) > 0 {
		return primitive.NewObjectIDFromTimestamp(timestamp[0])
	}

	return primitive.NewObjectID()
}

// IsHex will assess whether the provided string is a valid hex encoded
// object id.
func IsHex(str string) bool {
	_, err := FromHex(str)
	return err == nil
}

// FromHex will convert the provided string to an object id.
func FromHex(str string) (ID, error) {
	id, err := primitive.ObjectIDFromHex(str)
	return id, xo.W(err)
}

// MustFromHex will convert the provided string to an object id and panic if
// the string is not a valid object id.
func MustFromHex(str string) ID {
	id, err := FromHex(str)
	if err != nil {
		panic(err)
	}

	return id
}
