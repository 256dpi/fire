package ash

import (
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/stick"
)

// E is a short-hand function to create an enforcer.
func E(name string, m fire.Matcher, h fire.Handler) *Enforcer {
	return fire.C(name, 0, m, h) // TODO: Set stage?
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
// context. It should be used stand-alone if the presented candidate has full
// access to the data (.e.g a superuser) or in an authorizer chain to delegate
// authorization to the next authorizer.
func GrantAccess() *Enforcer {
	return E("ash/GrantAccess", fire.All(), func(_ *fire.Context) error {
		return nil
	})
}

// DenyAccess will enforce the authorization by directly returning an access
// denied error. It should be used if the operation should not be authorized in
// any case (.e.g a candidate accessing a resource he has clearly no access to).
//
// Note: Usually access is denied by returning no enforcers. This enforcer should
// only be returned to immediately stop the authorization process and prevent
// other enforcers from authorizing the operation.
func DenyAccess() *Enforcer {
	return E("ash/DenyAccess", fire.All(), func(_ *fire.Context) error {
		return fire.ErrAccessDenied.Wrap()
	})
}

// AddFilter will enforce the authorization by adding the passed filter to the
// Filter query of the context. It should be used if the candidate is allowed to
// access the resource in general, but some documents should be filtered out.
//
// Note: This enforcer cannot be used to authorize Create and CollectionAction
// operations.
func AddFilter(filter bson.M) *Enforcer {
	return E("ash/AddFilter", fire.Except(fire.Create|fire.CollectionAction), func(ctx *fire.Context) error {
		// assign specified filter
		ctx.Filters = append(ctx.Filters, filter)

		return nil
	})
}

// WhitelistReadableFields will enforce the authorization by making sure only the
// specified fields are returned for the client.
//
// Note: This enforcer cannot be used to authorize Delete, ResourceAction and
// CollectionAction operations.
func WhitelistReadableFields(fields ...string) *Enforcer {
	return E("ash/WhitelistReadableFields", fire.Except(fire.Delete|fire.ResourceAction|fire.CollectionAction), func(ctx *fire.Context) error {
		// set new list
		ctx.ReadableFields = stick.Intersect(ctx.ReadableFields, fields)

		return nil
	})
}

// SetReadableFieldsGetter will enforce the authorization by setting the
// specified readable fields getter.
//
// Note: This enforcer cannot be used to authorize Delete, ResourceAction and
// CollectionAction operations.
func SetReadableFieldsGetter(fn func(ctx *fire.Context, model coal.Model) []string) *Enforcer {
	return E("ash/SetReadableFieldsGetter", fire.Except(fire.Delete|fire.ResourceAction|fire.CollectionAction), func(ctx *fire.Context) error {
		// check readable fields getter
		if ctx.GetReadableFields != nil {
			return xo.F("existing readable fields getter")
		}

		// set readable fields getter
		ctx.GetReadableFields = func(model coal.Model) []string {
			return fn(ctx, model)
		}

		return nil
	})
}

// WhitelistWritableFields will enforce the authorization by making sure only the
// specified fields can be changed by the client.
//
// Note: This enforcer can only be used to authorize Create and Update operations.
func WhitelistWritableFields(fields ...string) *Enforcer {
	return E("ash/WhitelistWritableFields", fire.Only(fire.Create|fire.Update), func(ctx *fire.Context) error {
		// set new list
		ctx.WritableFields = stick.Intersect(ctx.WritableFields, fields)

		return nil
	})
}

// SetWritableFieldsGetter will enforce the authorization by setting the
// specified writable fields getter.
//
// Note: This enforcer can only be used to authorize Create and Update operations.
func SetWritableFieldsGetter(fn func(ctx *fire.Context, model coal.Model) []string) *Enforcer {
	return E("ash/SetReadableFieldsGetter", fire.Only(fire.Create|fire.Update), func(ctx *fire.Context) error {
		// check writable fields getter
		if ctx.GetWritableFields != nil {
			return xo.F("existing writable fields getter")
		}

		// set writable fields getter
		ctx.GetWritableFields = func(model coal.Model) []string {
			return fn(ctx, model)
		}

		return nil
	})
}

// WhitelistReadableProperties will enforce the authorization by making sure only
// the specified properties are returned for the client.
//
// Note: This enforcer cannot be used to authorize Delete, ResourceAction and
// CollectionAction operations.
func WhitelistReadableProperties(fields ...string) *Enforcer {
	return E("ash/WhitelistReadableProperties", fire.Except(fire.Delete|fire.ResourceAction|fire.CollectionAction), func(ctx *fire.Context) error {
		// set new list
		ctx.ReadableProperties = stick.Intersect(ctx.ReadableProperties, fields)

		return nil
	})
}

// SetReadablePropertiesGetter will enforce the authorization by setting the
// specified readable properties getter.
//
// Note: This enforcer cannot be used to authorize Delete, ResourceAction and
// CollectionAction operations.
func SetReadablePropertiesGetter(fn func(ctx *fire.Context, model coal.Model) []string) *Enforcer {
	return E("ash/SetReadablePropertiesGetter", fire.Except(fire.Delete|fire.ResourceAction|fire.CollectionAction), func(ctx *fire.Context) error {
		// check readable properties getter
		if ctx.GetReadableProperties != nil {
			return xo.F("existing readable properties getter")
		}

		// set readable properties getter
		ctx.GetReadableProperties = func(model coal.Model) []string {
			return fn(ctx, model)
		}

		return nil
	})
}

// AddRelationshipFilter will enforce the authorization by adding the passed
// relationship filter to the RelationshipFilter field of the context. It should
// be used if the candidate is allowed to access the relationship in general, but
// some documents should be filtered out.
//
// Note: This enforcer cannot be used to authorize Create and CollectionAction
// operations.
func AddRelationshipFilter(rel string, filter bson.M) *Enforcer {
	return E("ash/AddRelationshipFilter", fire.Except(fire.Create|fire.CollectionAction), func(ctx *fire.Context) error {
		// append filter
		ctx.RelationshipFilters[rel] = append(ctx.RelationshipFilters[rel], filter)

		return nil
	})
}
