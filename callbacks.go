package fire

import (
	"errors"

	"gopkg.in/mgo.v2/bson"
)

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

// The DependentResourcesValidator counts documents in the supplied collections
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
			n, err := ctx.DB.C(coll).Find(bson.M{field: ctx.ID}).Limit(1).Count()
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

// The VerifyReferencesValidator makes sure all references in the document are
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
