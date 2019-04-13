package coal

import (
	"fmt"
	"strings"
	"time"

	"github.com/globalsign/mgo/bson"
)

// C is a short-hand function to extract the collection of a model.
func C(m Model) string {
	return Init(m).Meta().Collection
}

// F is a short-hand function to extract the database BSON field name of a model
// field. Additionally, it supports the "-" prefix for retrieving sort keys.
//
// Note: F will panic if no field has been found.
func F(m Model, field string) string {
	// check if prefixed
	prefixed := strings.HasPrefix(field, "-")

	// remove prefix
	if prefixed {
		field = strings.TrimLeft(field, "-")
	}

	// find field
	f := Init(m).Meta().Fields[field]
	if f == nil {
		panic(fmt.Sprintf(`coal: field "%s" not found on "%s"`, field, m.Meta().Name))
	}

	// get field
	bsonField := f.BSONField

	// prefix field again
	if prefixed {
		bsonField = "-" + bsonField
	}

	return bsonField
}

// A is a short-hand function to extract the attribute JSON key of a model field.
//
// Note: A will panic if no field has been found.
func A(m Model, field string) string {
	// find field
	f := Init(m).Meta().Fields[field]
	if f == nil {
		panic(fmt.Sprintf(`coal: field "%s" not found on "%s"`, field, m.Meta().Name))
	}

	return f.JSONKey
}

// R is a short-hand function to extract the relationship name of a model field.
//
// Note: R will panic if no field has been found.
func R(m Model, field string) string {
	// find field
	f := Init(m).Meta().Fields[field]
	if f == nil {
		panic(fmt.Sprintf(`coal: field "%s" not found on "%s"`, field, m.Meta().Name))
	}

	return f.RelName
}

// L is a short-hand function to lookup a flagged field of a model.
//
// Note: L will panic if multiple flagged fields have been found or force is
// requested and no flagged field has been found.
func L(m Model, flag string, force bool) string {
	// lookup fields
	fields, _ := Init(m).Meta().FlaggedFields[flag]
	if len(fields) > 1 || (force && len(fields) == 0) {
		panic(fmt.Sprintf(`coal: no or multiple fields flagged as "%s" found on "%s"`, flag, m.Meta().Name))
	}

	// return name if found
	if len(fields) > 0 {
		return fields[0].Name
	}

	return ""
}

// P is a short-hand function to get a pointer of the passed object id.
func P(id bson.ObjectId) *bson.ObjectId {
	return &id
}

// N is a short-hand function to get a typed nil object id pointer.
func N() *bson.ObjectId {
	return nil
}

// T is a short-hand function to get a pointer of a timestamp.
func T(t time.Time) *time.Time {
	return &t
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

// Contains returns true if a list of object ids contains the specified id.
func Contains(list []bson.ObjectId, id bson.ObjectId) bool {
	for _, item := range list {
		if item == id {
			return true
		}
	}

	return false
}

// Includes returns true if a list of object ids includes another list of object
// ids.
func Includes(all, subset []bson.ObjectId) bool {
	for _, item := range subset {
		if !Contains(all, item) {
			return false
		}
	}

	return true
}

// Require will check if the specified flags are set on the specified model and
// panic if one is missing.
func Require(m Model, flags ...string) {
	// check all flags
	for _, f := range flags {
		L(m, f, true)
	}
}
