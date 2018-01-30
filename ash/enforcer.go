package ash

import (
	"github.com/256dpi/fire"

	"gopkg.in/mgo.v2/bson"
)

// E is a short-hand function to create an enforcer.
func E(name string, m fire.Matcher, h fire.Handler) *Enforcer {
	return fire.C(name, m, h)
}

// An Enforcer is returned by an Authorizer to enforce the previously inspected
// Authorization.
//
// Enforcers should only return errors if the operation is clearly not allowed for
// the presented candidate and that this information is general knowledge (e.g.
// API documentation). In order to prevent the leakage of implementation details
// the enforcer should mutate the context's Query field to hide existing data
// from the candidate.
type Enforcer = fire.Callback

// GrantAccess will enforce the authorization without any changes to the
// context. It should be used if the presented candidate has full access to the
// data (.e.g a superuser).
func GrantAccess() *Enforcer {
	return E("ash/GrantAccess", fire.All(), func(_ *fire.Context) error {
		return nil
	})
}

// DenyAccess will enforce the authorization by directly returning an access
// denied error. It should be used if the operation should not be authorized in
// any case (.e.g a candidate accessing a resource he has clearly no access to).
//
// Note: Usually access is denied by returning no enforcer. This enforcer should
// only be returned to immediately stop the authorization process and prevent
// other enforcers from authorizing the operation.
func DenyAccess() *Enforcer {
	return E("ash/DenyAccess", fire.All(), func(_ *fire.Context) error {
		return fire.ErrAccessDenied
	})
}

// AddFilter will enforce the authorization by adding the passed filter to the
// Filter query of the context. It should be used if the candidate is allowed to
// access the resource in general, but some records should be filtered out.
func AddFilter(filter bson.M) *Enforcer {
	return E("ash/AddFilter", fire.All(), func(ctx *fire.Context) error {
		// assign specified filter
		ctx.Filters = append(ctx.Filters, filter)

		return nil
	})
}

// WhitelistFields will enforce the authorization by making sure only the specified
// fields are returned for the client. If fields are existing it will only remove
// fields not present in the specified list.
func WhitelistFields(fields ...string) *Enforcer {
	return E("ash/WhitelistFields", fire.All(), func(ctx *fire.Context) error {
		// just set fields if not set yet
		if len(ctx.Fields) == 0 {
			ctx.Fields = fields
			return nil
		}

		// collect whitelisted fields
		var list []string
		for _, field := range ctx.Fields {
			if fire.Contains(fields, field) {
				list = append(list, field)
			}
		}

		// set new list
		ctx.Fields = list

		return nil
	})
}
