package ash

import (
	"reflect"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// TODO: Field access authorization should be clearer, especially the static
//  fields used for List and Create operations.

// PolicyDataKey is the key used to store policies.
const PolicyDataKey = "ash:policy"

// Policy defines an authorization policy.
type Policy struct {
	// Access defines the general access.
	Access Access

	// Actions defines the allowed actions.
	Actions map[string]bool

	// The default fields used to determine the field access level. If the
	// getter is set, these will only be used to establish valid filters and
	// sorters during the fire.List operation authorizer stage, as well as the
	// writable fields during the fire.Create operation, otherwise the model
	// specific fields are used instead.
	Fields AccessTable

	// GetFilter is called to obtain the general resource access filter. This
	// filter is used to narrow down accessible resources in all operations
	// except fire.Create and fire.CollectionAction operations.
	GetFilter func(ctx *fire.Context) bson.M

	// VerifyID is called to for every direct model lookup to verify resource
	// level access. This function is called for all operations except fire.List,
	// fire.Create and fire.CollectionAction.
	VerifyID func(ctx *fire.Context, id coal.ID) Access

	// VerifyModel is called for every model load from the database to determine
	// resource level access. This function is called for all operations except
	// fire.Create and fire.CollectionAction.
	//
	// Note: The verification is deferred to the fire.Verifier stage.
	VerifyModel func(ctx *fire.Context, model coal.Model) Access

	// VerifyCreate and VerifyUpdate determine resource level access after all
	// modification have been applied. This function is called for the
	// fire.Create and fire.Update operation.
	//
	// Note: The verification is deferred to the fire.Validator stage.
	VerifyCreate func(ctx *fire.Context, model coal.Model) bool
	VerifyUpdate func(ctx *fire.Context, model coal.Model) bool

	// GetFields is called for every model to determine the field level access.
	// The policy should refrain from creating a new map for every request and
	// instead pre-allocate possible combinations and return those. The function
	// is called for all operations except fire.Delete, fire.CollectionAction and
	// fire.ResourceAction.
	GetFields func(ctx *fire.Context, model coal.Model) AccessTable

	// GetProperties is called for every model to determine the property level
	// access. The policy should refrain from creating a map for every request
	// and instead pre-allocate possible combinations and return those. The
	// function is called for all operations except fire.Delete,
	// fire.CollectionAction and fire.ResourceAction.
	GetProperties func(ctx *fire.Context, model coal.Model) AccessTable
}

// Selector is the function run to select a policy.
type Selector func(Identity) *Policy

// Select will run the provided function to select a policy for the supplied
// identity.
func Select(selector Selector) *fire.Callback {
	return fire.C("ash/Selector", fire.Authorizer, fire.All(), func(ctx *fire.Context) error {
		// get identity
		identity := ctx.Data[IdentityDataKey]
		if identity == nil {
			return fire.ErrAccessDenied.Wrap()
		}

		// run selector
		policy := selector(identity)

		// check policy
		if policy == nil {
			return nil
		}

		// check stored
		if ctx.Data[PolicyDataKey] != nil {
			return xo.F("existing policy")
		}

		// store policy
		ctx.Data[PolicyDataKey] = policy

		return nil
	})
}

// SelectMatch will match the provided identity and on success use the provided
// factory to create a policy.
func SelectMatch(identity Identity, policy func(Identity) *Policy) *fire.Callback {
	// get type
	typ := reflect.TypeOf(identity)

	return Select(func(identity Identity) *Policy {
		// check type
		if typ != reflect.TypeOf(identity) {
			return nil
		}

		return policy(identity)
	})
}

// SelectPublic will math the public identity and use the provided factory to
// create a policy.
func SelectPublic(fn func() *Policy) *fire.Callback {
	return SelectMatch(&PublicIdentity{}, func(Identity) *Policy {
		return fn()
	})
}

// Execute will execute the selected policy or deny access.
func Execute() *fire.Callback {
	// prepare matchers
	getFilterMatcher := fire.Except(fire.Create | fire.CollectionAction)
	verifyIDMatcher := fire.Except(fire.List | fire.Create | fire.CollectionAction)
	verifyModelMatcher := fire.Except(fire.Create | fire.CollectionAction)
	verifyCreateMatcher := fire.Only(fire.Create)
	verifyUpdateMatcher := fire.Only(fire.Update)
	getFieldsMatcher := fire.Except(fire.Delete | fire.CollectionAction | fire.ResourceAction)

	// prepare access tables
	genericAccess := map[fire.Operation]Access{
		fire.List:           List,
		fire.Find:           Find,
		fire.Create:         Create,
		fire.Update:         Update,
		fire.Delete:         Delete,
		fire.ResourceAction: Find,
	}
	readAccess := map[fire.Operation]Access{
		fire.List:   List,
		fire.Find:   Find,
		fire.Create: Find,
		fire.Update: Find,
	}
	writeAccess := map[fire.Operation]Access{
		fire.Create: Create,
		fire.Update: Update,
	}

	return fire.C("ash/Execute", fire.Authorizer, fire.All(), func(ctx *fire.Context) error {
		// get policy
		policy, _ := ctx.Data[PolicyDataKey].(*Policy)
		if policy == nil {
			return fire.ErrAccessDenied.Wrap()
		}

		// check access
		access := genericAccess[ctx.Operation]
		if policy.Access&access != access {
			return fire.ErrAccessDenied.Wrap()
		}

		// apply filter if available
		if getFilterMatcher(ctx) && policy.GetFilter != nil {
			ctx.Filters = append(ctx.Filters, policy.GetFilter(ctx))
		}

		// verify action access
		if ctx.Operation.Action() {
			// get action
			action := ctx.JSONAPIRequest.CollectionAction
			if ctx.Operation == fire.ResourceAction {
				action = ctx.JSONAPIRequest.ResourceAction
			}

			// check action
			if !policy.Actions[action] {
				return fire.ErrAccessDenied.Wrap()
			}
		}

		// verify id if available
		if verifyIDMatcher(ctx) && policy.VerifyID != nil {
			// get access
			access := policy.VerifyID(ctx, ctx.Selector["_id"].(coal.ID))

			// check access
			if access&genericAccess[ctx.Operation] == 0 {
				return fire.ErrAccessDenied.Wrap()
			}
		}

		// verify model if available
		if verifyModelMatcher(ctx) && policy.VerifyModel != nil {
			ctx.Defer(fire.C("ash/Execute-VerifyModel", fire.Verifier, verifyModelMatcher, func(ctx *fire.Context) error {
				// get required access
				reqAccess := genericAccess[ctx.Operation]

				// check access
				if ctx.Operation == fire.List {
					for _, model := range ctx.Models {
						if policy.VerifyModel(ctx, model)&reqAccess == 0 {
							return fire.ErrAccessDenied.Wrap()
						}
					}
				} else {
					if policy.VerifyModel(ctx, ctx.Model)&reqAccess == 0 {
						return fire.ErrAccessDenied.Wrap()
					}
				}

				return nil
			}))
		}

		// verify create if available
		if verifyCreateMatcher(ctx) && policy.VerifyCreate != nil {
			ctx.Defer(fire.C("ash/Execute-VerifyCreate", fire.Validator, verifyCreateMatcher, func(ctx *fire.Context) error {
				// check access
				if !policy.VerifyCreate(ctx, ctx.Model) {
					return fire.ErrAccessDenied.Wrap()
				}

				return nil
			}))
		}

		// verify update if available
		if verifyUpdateMatcher(ctx) && policy.VerifyUpdate != nil {
			ctx.Defer(fire.C("ash/Execute-VerifyUpdate", fire.Validator, verifyUpdateMatcher, func(ctx *fire.Context) error {
				// check access
				if !policy.VerifyUpdate(ctx, ctx.Model) {
					return fire.ErrAccessDenied.Wrap()
				}

				return nil
			}))
		}

		// collect fields
		readableFields := policy.Fields.Collect(readAccess[ctx.Operation])
		writableFields := policy.Fields.Collect(writeAccess[ctx.Operation])

		// set intersections of fields
		ctx.ReadableFields = stick.Intersect(ctx.ReadableFields, readableFields)
		ctx.WritableFields = stick.Intersect(ctx.WritableFields, writableFields)

		// set fields getters if available
		if getFieldsMatcher(ctx) && policy.GetFields != nil {
			ctx.GetReadableFields = func(model coal.Model) []string {
				if model == nil {
					return readableFields
				}
				return policy.GetFields(ctx, model).Collect(readAccess[ctx.Operation])
			}
			ctx.GetWritableFields = func(model coal.Model) []string {
				if ctx.Operation == fire.Create {
					return writableFields
				}
				return policy.GetFields(ctx, model).Collect(writeAccess[ctx.Operation])
			}
		}

		// set properties getter if available
		if getFieldsMatcher(ctx) && policy.GetProperties != nil {
			ctx.GetReadableProperties = func(model coal.Model) []string {
				return policy.GetProperties(ctx, model).Collect(readAccess[ctx.Operation])
			}
		}

		return nil
	})
}
