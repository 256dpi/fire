package fire

import (
	"errors"

	"gopkg.in/mgo.v2/bson"
)

type Callback func(*Context) (error, error)

// Combine combines multiple callbacks to one.
func Combine(callbacks... Callback) Callback {
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
