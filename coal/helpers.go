package coal

import (
	"fmt"
	"strings"
	"time"

	"github.com/256dpi/lungo/bsonkit"
	"github.com/256dpi/lungo/mongokit"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/stick"
)

// F is a shorthand function to extract the BSON key of a model field.
// Additionally, it supports the "-" prefix for retrieving sort keys.
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
	field, err := NewTranslator(m).Field(field)
	if err != nil {
		panic("coal: " + err.Error())
	}

	// prefix field again
	if prefixed {
		field = "-" + field
	}

	return field
}

// L is a shorthand function to look up a flagged field of a model.
//
// Note: L will panic if multiple flagged fields have been found or force is
// requested and no flagged field has been found.
func L(m Model, flag string, force bool) string {
	// lookup fields
	fields := GetMeta(m).FlaggedFields[flag]
	if len(fields) > 1 || (force && len(fields) == 0) {
		panic(fmt.Sprintf(`coal: no or multiple fields flagged as "%s" on "%s"`, flag, GetMeta(m).Name))
	}

	// return name if found
	if len(fields) > 0 {
		return fields[0].Name
	}

	return ""
}

// T is a helper function to construct the BSON key for a tag.
func T(name string) string {
	return "_tg." + name
}

// TV is a helper function to construct the BSON key for a tag value.
func TV(name string) string {
	return T(name) + ".v"
}

// TE is a helper function to construct the BSON key for a tag expiry.
func TE(name string) string {
	return T(name) + ".e"
}

// TF is a helper function to construct the BSON filter for a tag expiry. Only
// tags with a non-zero expiry can become expired.
func TF(expired bool) bson.M {
	if expired {
		return bson.M{
			"$lt": time.Now(),
		}
	}
	return bson.M{
		"$not": bson.M{
			"$lt": time.Now(),
		},
	}
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
		var value int32 = 1
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

// ReverseSort is a helper function to revers a sort.
func ReverseSort(sort []string) []string {
	// reverse sort
	newSort := make([]string, 0, len(sort))
	for _, key := range sort {
		if strings.HasPrefix(key, "-") {
			newSort = append(newSort, key[1:])
		} else {
			newSort = append(newSort, "-"+key)
		}
	}

	return newSort
}

// ToM converts a model to a bson.M including all database fields.
func ToM(model Model) bson.M {
	// prepare map
	m := bson.M{}

	// add all fields
	for name, field := range GetMeta(model).DatabaseFields {
		m[name] = stick.MustGet(model, field.Name)
	}

	return m
}

// ToD converts a model to a bson.D including all database fields.
func ToD(model Model) bson.D {
	// get fields
	fields := GetMeta(model).OrderedFields

	// prepare document
	d := make(bson.D, 0, len(fields))

	// add all fields
	for _, field := range fields {
		if field.BSONKey != "" {
			d = append(d, bson.E{
				Key:   field.BSONKey,
				Value: stick.MustGet(model, field.Name),
			})
		}
	}

	return d
}

// Apply will apply the provided update document to the specified model.
func Apply(model Model, update bson.M) error {
	// skip if update is empty
	if len(update) == 0 {
		return nil
	}

	// transform model
	modelDoc, err := bsonkit.Transform(model)
	if err != nil {
		return xo.W(err)
	}

	// transform update
	updateDoc, err := bsonkit.Transform(update)
	if err != nil {
		return xo.W(err)
	}

	// apply update
	_, err = mongokit.Apply(modelDoc, nil, updateDoc, false, nil)
	if err != nil {
		return xo.W(err)
	}

	// decode model
	err = bsonkit.Decode(modelDoc, model)
	if err != nil {
		return xo.W(err)
	}

	return nil
}
