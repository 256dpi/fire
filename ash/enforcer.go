package ash

import (
	"github.com/256dpi/fire"
	"gopkg.in/mgo.v2/bson"
)

// E is a short-hand function to create an enforcer.
func E(n string, cb fire.Callback) *Enforcer {
	return &Enforcer{
		Name:     n,
		Callback: cb,
	}
}

// An Enforcer is returned by an Authorizer to enforce the previously inspected
// Authorization.
//
// Enforcers should only return errors if the operation is clearly not allowed for
// the presented candidate and that this information is general knowledge (e.g.
// API documentation). In order to prevent the leakage of implementation details
// the enforcer should mutate the context's Query field to hide existing data
// from the candidate.
type Enforcer struct {
	Name     string
	Callback fire.Callback
}

// GrantAccess will enforce the authorization without any changes to the
// context. It should be used if the presented candidate has full access to the
// data (.e.g a superuser).
func GrantAccess() *Enforcer {
	return E("ash.GrantAccess", func(_ *fire.Context) error {
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
	return E("ash.DenyAccess", func(_ *fire.Context) error {
		return ErrAccessDenied
	})
}

// AddFilter will enforce the authorization by adding the passed filters to the
// Filter query of the context. It should be used if the candidate is allowed to
// access the resource in general, but some records should be filtered out.
//
// Note: This method will panic if used for Create and CollectionAction operation.
// You should test for this cases and use another enforcer.
func AddFilter(filters bson.M) *Enforcer {
	return E("ash.AddFilter", func(ctx *fire.Context) error {
		// panic on create and collection action
		if ctx.Operation == fire.Create || ctx.Operation == fire.CollectionAction {
			panic("ash: operation not supported")
		}

		// assign specified filters
		for key, value := range filters {
			ctx.Filter[key] = value
		}

		return nil
	})
}

// HideFilter will enforce the authorization by adding a falsy filter to the
// Filter query of the context, so that no records will be returned. It should be
// used if the requested resources or resource should be hidden from the candidate.
func HideFilter() *Enforcer {
	// TODO: Authorizers should be allowed to return ErrNotFound to trigger
	// an early ErrNotFound instead of manipulating the Query in crazy ways.

	return E("ash.HideFilter", func(ctx *fire.Context) error {
		// panic on create and collection action
		if ctx.Operation == fire.Create || ctx.Operation == fire.CollectionAction {
			panic("ash: operation not supported")
		}

		// assign specified filters
		ctx.Filter["___a_property_no_document_in_this_world_should_have"] = "value"

		return nil
	})
}
