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

// Unique is a helper to get a unique list of object ids.
func Unique(ids []ID) []ID {
	// check nil
	if ids == nil {
		return nil
	}

	// prepare table and result
	table := make(map[ID]bool)
	res := make([]ID, 0, len(ids))

	// add ids not in table
	for _, id := range ids {
		if _, ok := table[id]; !ok {
			table[id] = true
			res = append(res, id)
		}
	}

	return res
}

// Contains returns true if a list of object ids contains the specified id.
func Contains(list []ID, id ID) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}

	return false
}

// Includes returns true if a list of object ids includes another list of object
// ids.
func Includes(all, subset []ID) bool {
	for _, item := range subset {
		if !Contains(all, item) {
			return false
		}
	}

	return true
}

// Union will merge all list and remove duplicates.
func Union(lists ...[]ID) []ID {
	// check lists
	if len(lists) == 0 {
		return nil
	}

	// sum length and check nil
	var sum int
	var nonNil bool
	for _, l := range lists {
		sum += len(l)
		if l != nil {
			nonNil = true
		}
	}
	if !nonNil {
		return nil
	}

	// prepare table and result
	table := make(map[ID]bool, sum)
	res := make([]ID, 0, sum)

	// add items not present in table
	for _, list := range lists {
		for _, item := range list {
			if _, ok := table[item]; !ok {
				table[item] = true
				res = append(res, item)
			}
		}
	}

	return res
}

// Subtract will return a list with items that are only part of the first list.
func Subtract(listA, listB []ID) []ID {
	// check nil
	if listA == nil {
		return nil
	}

	// prepare new list
	list := make([]ID, 0, len(listA))

	// add items that are not in second list
	for _, item := range listA {
		if !Contains(listB, item) {
			list = append(list, item)
		}
	}

	return list
}

// Intersect will return a list with items that are part of both lists.
func Intersect(listA, listB []ID) []ID {
	// check nil
	if listA == nil || listB == nil {
		return nil
	}

	// prepare new list
	list := make([]ID, 0, len(listA))

	// add items that are part of both lists
	for _, item := range listA {
		if Contains(listB, item) {
			list = append(list, item)
		}
	}

	return list
}
