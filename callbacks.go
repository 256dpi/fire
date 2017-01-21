package fire

import (
	"errors"
	"fmt"
	"reflect"

	"gopkg.in/mgo.v2/bson"
)

// A Callback is called during execution of a controller.
//
// Note: If the callback returns an error wrapped using Fatal() the API returns
// an InternalServerError status and the error will be logged. All other errors
// are serialized to an error object and returned.
type Callback func(*Context) error

type fatalError struct {
	err error
}

// Fatal wraps an error and marks it as fatal.
func Fatal(err error) error {
	return &fatalError{
		err: err,
	}
}

func (err *fatalError) Error() string {
	return err.err.Error()
}

func isFatal(err error) bool {
	_, ok := err.(*fatalError)
	return ok
}

type value int

// NoDefault marks the specified field to have no default that needs to be
// enforced while executing the ProtectedAttributesValidator.
const NoDefault value = iota

// Only will return a callback that runs the specified callback only when one
// of the supplied actions match.
func Only(cb Callback, actions ...Action) Callback {
	return func(ctx *Context) error {
		// check action
		for _, a := range actions {
			if a == ctx.Action {
				return cb(ctx)
			}
		}

		return nil
	}
}

// Except will return a callback that runs the specified callback only when none
// of the supplied actions match.
func Except(cb Callback, actions ...Action) Callback {
	return func(ctx *Context) error {
		// check action
		for _, a := range actions {
			if a == ctx.Action {
				return nil
			}
		}

		return cb(ctx)
	}
}

// ModelValidator performs a validation of the model using the Validate
// function.
func ModelValidator() Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// TODO: Add error source pointer.

		return Validate(ctx.Model)
	}
}

// ProtectedAttributesValidator compares protected attributes against their
// default (during Create) or stored value (during Update) and returns an error
// if they have been changed.
//
// Attributes are defined by passing pairs of fields and default values:
//
//	ProtectedAttributesValidator(map[string]interface{}{
//		F(&Post{}, "Title"): NoDefault, // can only be set during Create
//		F(&Post{}, "Link"):  "",        // is fixed and cannot be changed
//	})
//
// The special NoDefault value can be provided to skip the default enforcement
// on Create.
//
func ProtectedAttributesValidator(attributes map[string]interface{}) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		if ctx.Action == Create {
			// check all attributes
			for field, def := range attributes {
				// skip fields that have no default
				if def == NoDefault {
					continue
				}

				// check equality
				if !reflect.DeepEqual(ctx.Model.MustGet(field), def) {
					return errors.New("Field " + field + " is protected")
				}
			}
		}

		if ctx.Action == Update {
			// read the original
			original, err := ctx.Original()
			if err != nil {
				return err
			}

			// check all attributes
			for field := range attributes {
				// check equality
				if !reflect.DeepEqual(ctx.Model.MustGet(field), original.MustGet(field)) {
					return errors.New("Field " + field + " is protected")
				}
			}
		}

		return nil
	}
}

// DependentResourcesValidator counts documents in the supplied collections
// and returns an error if some get found. This callback is meant to protect
// resources from breaking relations when requested to be deleted.
//
// Dependent resources are defined by passing pairs of collections and database
// fields that hold the current models id:
//
//	DependentResourcesValidator(map[string]string{
//		C(&Post{}): F(&Post{}, "Author"),
//		C(&Comment{}): F(&Post{}, "Author"),
//	})
//
func DependentResourcesValidator(resources map[string]string) Callback {
	return func(ctx *Context) error {
		// only run validator on Delete
		if ctx.Action != Delete {
			return nil
		}

		// check all relations
		for coll, field := range resources {
			// count referencing documents
			n, err := ctx.Store.DB().C(coll).Find(bson.M{
				field: ctx.Query["_id"],
			}).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}

			// immediately return if a document is found
			if n == 1 {
				return errors.New("Resource has dependent resources")
			}
		}

		// pass validation
		return nil
	}
}

// VerifyReferencesValidator makes sure all references in the document are
// existing by counting the references on the related collections.
//
// References are defined by passing pairs of database fields and collections of
// models whose ids might be referenced on the current model:
//
//	VerifyReferencesValidator(map[string]string{
//		F(&Comment{}, "Post"): C(&Post{}),
//		F(&Comment{}, "Author"): C(&User{}),
//	})
//
func VerifyReferencesValidator(references map[string]string) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// check all references
		for field, collection := range references {
			// read referenced id
			id := ctx.Model.MustGet(field)

			// continue if reference is not set
			if oid, ok := id.(*bson.ObjectId); ok && oid == nil {
				continue
			}

			// count entities in database
			n, err := ctx.Store.DB().C(collection).FindId(id).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}

			// check for existence
			if n != 1 {
				return errors.New("Missing required relationship " + field)
			}
		}

		// pass validation
		return nil
	}
}

// MatchingReferencesValidator compares the model with a related model and
// checks if certain references are shared.
//
// The target model is defined by passing its collection and the referencing
// field on the current model. The matcher is defined by passing pairs of
// database fields on the target and current model:
//
//	MatchingReferencesValidator(C(&Blog{}), F(&Post{}, "Blog"), map[string]string{
//		F(&Blog{}, "Owner"): F(&Post{}, "Owner"),
//	})
//
func MatchingReferencesValidator(collection, reference string, matcher map[string]string) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// get main reference
		id := ctx.Model.MustGet(reference)

		// continue if reference is not set
		if oid, ok := id.(*bson.ObjectId); ok && oid == nil {
			return nil
		}

		// prepare query
		query := bson.M{
			"_id": id,
		}

		// add other references
		for targetField, modelField := range matcher {
			id := ctx.Model.MustGet(modelField)

			// abort if reference is missing
			if oid, ok := id.(*bson.ObjectId); ok && oid == nil {
				return errors.New("Missing ID")
			}

			query[targetField] = id
		}

		// query db
		n, err := ctx.Store.DB().C(collection).Find(query).Limit(1).Count()
		if err != nil {
			return Fatal(err)
		}

		// return error if document is missing
		if n == 0 {
			return errors.New("References do not match")
		}

		return nil
	}
}

// UniqueAttributeValidator ensures that the specified attribute of the
// controllers Model will remain unique among the specified filters.
//
// The unique attribute is defines as the first argument. Filters are defined
// by passing a list of database fields:
//
//	UniqueAttributeValidator(F(&Blog{}, "Name"), F(&Blog{}, "Creator"))
//
func UniqueAttributeValidator(uniqueAttribute string, filters ...string) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// check if field has changed
		if ctx.Action == Update {
			// get original model
			original, err := ctx.Original()
			if err != nil {
				return err
			}

			// return if field has not been changed
			if ctx.Model.MustGet(uniqueAttribute) == original.MustGet(uniqueAttribute) {
				return nil
			}
		}

		// prepare query
		query := bson.M{
			uniqueAttribute: ctx.Model.MustGet(uniqueAttribute),
		}

		// add filters
		for _, field := range filters {
			query[field] = ctx.Model.MustGet(field)
		}

		// count
		n, err := ctx.Store.C(ctx.Model).Find(query).Limit(1).Count()
		if err != nil {
			return Fatal(err)
		} else if n != 0 {
			return fmt.Errorf("Attribute %s is not unique", uniqueAttribute)
		}

		return nil
	}
}
