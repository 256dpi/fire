package fire

import (
	"errors"

	"gopkg.in/mgo.v2/bson"
)

// A Callback allows further extensibility and customisation of the API.
//
// Note: The first return value is the userError which will be serialized to the
// jsonapi error object. The second return value is the system error that appears
// just in the logs.
type Callback func(*Context) (error, error)

// Combine combines multiple callbacks to one.
func Combine(callbacks ...Callback) Callback {
	return func(ctx *Context) (error, error) {
		// call all callbacks
		for _, cb := range callbacks {
			err, sysErr := cb(ctx)

			// return early if an error occurs
			if err != nil || sysErr != nil {
				return err, sysErr
			}
		}

		return nil, nil
	}
}

// DependentResourcesValidator counts documents in the supplied collections
// and returns an error if some get found. This callback is meant to protect
// resources from breaking relations when requested to be deleted.
func DependentResourcesValidator(relations map[string]string) Callback {
	return func(ctx *Context) (error, error) {
		// only run validator on Delete
		if ctx.Action != Delete {
			return nil, nil
		}

		// check all relations
		for coll, field := range relations {
			// count referencing documents
			n, err := ctx.DB.C(coll).Find(bson.M{field: ctx.Query["_id"]}).Limit(1).Count()
			if err != nil {
				return nil, err
			}

			// immediately return if a document is found
			if n == 1 {
				return errors.New("resource has dependent resources"), nil
			}
		}

		// pass validation
		return nil, nil
	}
}

// VerifyReferencesValidator makes sure all references in the document are
// existing by counting on the related collections.
func VerifyReferencesValidator(relations map[string]string) Callback {
	return func(ctx *Context) (error, error) {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil, nil
		}

		// check all relations
		for relation, collection := range relations {
			// read referenced fire.Resource id
			id := ctx.Model.ReferenceID(relation)
			if id == nil {
				continue
			}

			// count entities in database
			n, err := ctx.DB.C(collection).FindId(id).Limit(1).Count()
			if err != nil {
				return nil, err
			}

			// check for existence
			if n != 1 {
				return errors.New("missing required relationship " + relation), nil
			}
		}

		// pass validation
		return nil, nil
	}
}

// MatchingReferencesValidator compares the model with a referencing relation and
// checks if they share other relations.
func MatchingReferencesValidator(collection, referencingRelation string, matcher map[string]string) Callback {
	return func(ctx *Context) (error, error) {
		// only run validator on Create and Update
		if ctx.Action != Create && ctx.Action != Update {
			return nil, nil
		}

		// get main reference
		id := ctx.Model.ReferenceID(referencingRelation)
		if id == nil {
			// continue if relation is not set
			return nil, nil
		}

		// prepare query
		query := bson.M{
			"_id": *id,
		}

		// add other references
		for field, relation := range matcher {
			id := ctx.Model.ReferenceID(relation)
			if id == nil {
				return errors.New("missing id"), nil
			}

			query[field] = *id
		}

		// query db
		n, err := ctx.DB.C(collection).Find(query).Limit(1).Count()
		if err != nil {
			return nil, err
		}

		// return error if document is missing
		if n == 0 {
			return errors.New("references do not match"), nil
		}

		return nil, nil
	}
}
