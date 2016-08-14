package fire

import (
	"errors"

	"gopkg.in/mgo.v2/bson"
)

// A Callback is can be an Authorizer or Validator an is called during accessing
// a resource via the API.
//
// Note: If the callback returns an error wrapped using Fatal() the API returns
// an InternalServerError status and the error will be logged. All other errors
// are serialized to an error object and returned.
type Callback func(*Context) error

// SM is a general purpose string map.
type SM map[string]string

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
	return "fatal: " + err.err.Error()
}

func isFatal(err error) bool {
	_, ok := err.(*fatalError)
	return ok
}

// Combine combines multiple callbacks to one.
//
// Note: Execution will be stopped if a callback returns and error.
func Combine(callbacks ...Callback) Callback {
	return func(ctx *Context) error {
		// call all callbacks
		for _, cb := range callbacks {
			err := cb(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// DependentResourcesValidator counts documents in the supplied collections
// and returns an error if some get found. This callback is meant to protect
// resources from breaking relations when requested to be deleted.
//
// Resources are defined by passing pairs of collections and fields where the
// field must be a database field of the target resource model:
//
//		DependentResourcesValidator(SM{
// 			"posts": "user_id",
//			"comments": "user_id",
// 		})
//
func DependentResourcesValidator(resources SM) Callback {
	return func(ctx *Context) error {
		// only run validator on Delete
		if ctx.Action != Delete {
			return nil
		}

		// check all relations
		for coll, field := range resources {
			// count referencing documents
			n, err := ctx.DB.C(coll).Find(bson.M{field: ctx.Query["_id"]}).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}

			// immediately return if a document is found
			if n == 1 {
				return errors.New("resource has dependent resources")
			}
		}

		// pass validation
		return nil
	}
}

// VerifyReferencesValidator makes sure all references in the document are
// existing by counting on the related collections.
//
// References are defined by passing pairs of fields and collections where the
// field must be a database field on the resource model:
//
//		VerifyReferencesValidator(SM{
// 			"post_id": "posts",
//			"user_id": "users",
// 		})
//
func VerifyReferencesValidator(references SM) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// check all references
		for field, collection := range references {
			// read referenced id
			id := ctx.Model.Attribute(field)
			if id == nil {
				continue
			}

			// count entities in database
			n, err := ctx.DB.C(collection).FindId(id).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}

			// check for existence
			if n != 1 {
				return errors.New("missing required relationship " + field)
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
//		MatchingReferencesValidator("posts", "post_id", SM{
// 			"user_id": "user_id",
// 		})
//
func MatchingReferencesValidator(collection, reference string, matcher SM) Callback {
	return func(ctx *Context) error {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil
		}

		// get main reference
		id := ctx.Model.Attribute(reference)
		if id == nil {
			// continue if relation is not set
			return nil
		}

		// prepare query
		query := bson.M{
			"_id": id,
		}

		// add other references
		for targetField, modelField := range matcher {
			id := ctx.Model.Attribute(modelField)
			if id == nil {
				return errors.New("missing id")
			}

			query[targetField] = id
		}

		// query db
		n, err := ctx.DB.C(collection).Find(query).Limit(1).Count()
		if err != nil {
			return Fatal(err)
		}

		// return error if document is missing
		if n == 0 {
			return errors.New("references do not match")
		}

		return nil
	}
}
