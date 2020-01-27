package coal

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
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
		panic(fmt.Sprintf(`coal: no or multiple fields flagged as "%s" on "%s"`, flag, m.Meta().Name))
	}

	// return name if found
	if len(fields) > 0 {
		return fields[0].Name
	}

	return ""
}

// T is a short-hand function to get a pointer of a timestamp.
func T(t time.Time) *time.Time {
	return &t
}

// Require will check if the specified flags are set on the specified model and
// panic if one is missing.
func Require(m Model, flags ...string) {
	// check all flags
	for _, f := range flags {
		L(m, f, true)
	}
}

// Sort is a helper function to compute a sort object based on a list of fields
// with dash prefixes for descending sorting.
func Sort(fields ...string) bson.D {
	// prepare sort
	var sort bson.D

	// add fields
	for _, field := range fields {
		// check if prefixed
		prefixed := strings.HasPrefix(field, "-")

		// remove prefix
		if prefixed {
			field = strings.TrimLeft(field, "-")
		}

		// prepare value
		value := 1
		if prefixed {
			value = -1
		}

		// add field
		sort = append(sort, bson.E{
			Key:   field,
			Value: value,
		})
	}

	return sort
}

// ToM converts a model to a bson.M including all database fields.
func ToM(model Model) bson.M {
	// prepare map
	m := bson.M{}

	// add all fields
	for name, field := range Init(model).Meta().DatabaseFields {
		m[name] = MustGet(model, field.Name)
	}

	return m
}

// ToD converts a model to a bson.D including all database fields.
func ToD(model Model) bson.D {
	// get fields
	fields := Init(model).Meta().OrderedFields

	// prepare document
	d := make(bson.D, 0, len(fields))

	// add all fields
	for _, field := range fields {
		if field.BSONField != "" {
			d = append(d, bson.E{
				Key:   field.BSONField,
				Value: MustGet(model, field.Name),
			})
		}
	}

	return d
}
