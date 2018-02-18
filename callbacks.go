package fire

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// C is a short-hand function to construct a callback. It will also add tracing
// code around the execution of the callback.
func C(name string, m Matcher, h Handler) *Callback {
	// panic if matcher or handler is not set
	if m == nil || h == nil {
		panic("fire: missing matcher or handler")
	}

	return &Callback{
		Matcher: m,
		Handler: func(ctx *Context) error {
			// begin trace
			ctx.Tracer.Push(name)

			// call handler
			err := h(ctx)
			if err != nil {
				return err
			}

			// finish trace
			ctx.Tracer.Pop()

			return nil
		},
	}
}

// Handler is function that takes a context, mutates is to modify the behaviour
// and response or return an error.
type Handler func(*Context) error

// Matcher is a function that makes an assessment of a context and decides whether
// a modification should be applied in the future.
type Matcher func(*Context) bool

// All will match all contexts.
func All() Matcher {
	return func(ctx *Context) bool {
		return true
	}
}

// Only will match if the operation is present in the provided list.
func Only(ops ...Operation) Matcher {
	return func(ctx *Context) bool {
		// allow if operation is listed
		for _, op := range ops {
			if op == ctx.Operation {
				return true
			}
		}

		return false
	}
}

// Except will match if the operation is not present in the provided list.
func Except(ops ...Operation) Matcher {
	return func(ctx *Context) bool {
		// disallow if operation is listed
		for _, op := range ops {
			if op == ctx.Operation {
				return false
			}
		}

		return true
	}
}

// A Callback is called during the request processing flow of a controller.
//
// Note: If the callback returns an error wrapped using Fatal() the API returns
// an InternalServerError status and the error will be logged. All other errors
// are serialized to an error object and returned.
type Callback struct {
	// The matcher that decides whether the callback should be run.
	Matcher Matcher

	// The handler handler that gets executed with the context.
	//
	// If returned errors are marked with Safe() they will be included in the
	// returned JSON-API error.
	Handler Handler
}

// ErrAccessDenied can be returned by any callback to deny access.
var ErrAccessDenied = jsonapi.ErrorFromStatus(http.StatusUnauthorized, "access denied")

// BasicAuthorizer authorizes requests based on a simple credentials list.
func BasicAuthorizer(credentials map[string]string) *Callback {
	return C("fire/BasicAuthorizer", All(), func(ctx *Context) error {
		// check for credentials
		user, password, ok := ctx.HTTPRequest.BasicAuth()
		if !ok {
			return ErrAccessDenied
		}

		// check if credentials match
		if val, ok := credentials[user]; !ok || val != password {
			return ErrAccessDenied
		}

		return nil
	})
}

// ValidationError returns a JSON-API error that points to a specific attribute
// or relationship.
func ValidationError(pointer, detail string) error {
	return jsonapi.BadRequestPointer(detail, pointer)
}

// The ValidatableModel interface can be additionally implemented to provide
// a custom validation method that is used by the Validate function.
type ValidatableModel interface {
	coal.Model

	// The Validate method that should return normal errors about invalid fields.
	Validate() error
}

// ModelValidator performs a validation of the model using the Validate method.
func ModelValidator() *Callback {
	return C("fire/ModelValidator", Only(Create, Update), func(ctx *Context) error {
		// check model
		m, ok := ctx.Model.(ValidatableModel)
		if !ok {
			return fmt.Errorf("model is not validatable")
		}

		// validate model
		err := m.Validate()
		if err != nil {
			return err
		}

		return nil
	})
}

// TimestampValidator will set the specified timestamp fields on creation
// and update operations.
func TimestampValidator(createdAtField, updatedAtField string) *Callback {
	return C("fire/TimestampValidator", Only(Create, Update), func(ctx *Context) error {
		// set the created-at field if present and not already set
		if createdAtField != "" && ctx.Model.MustGet(createdAtField).(time.Time).IsZero() {
			ctx.Model.MustSet(createdAtField, time.Now())
		}

		// always set the updated-at field if present
		if updatedAtField != "" {
			ctx.Model.MustSet(updatedAtField, time.Now())
		}

		return nil
	})
}

// NoDefault marks the specified field to have no default that needs to be
// enforced while executing the ProtectedFieldsValidator.
const NoDefault noDefault = iota

type noDefault int

// ProtectedFieldsValidator compares protected fields against their default
// during Create (if provided) or stored value during Update and returns an error
// if they have been changed.
//
// Protected fields are defined by passing pairs of fields and default values:
//
//	fire.ProtectedFieldsValidator(map[string]interface{}{
//		"Title": NoDefault, // can only be set during Create
//		"Link":  "",        // default is fixed and cannot be changed
//	})
//
// The special NoDefault value can be provided to skip the default enforcement
// on Create.
func ProtectedFieldsValidator(pairs map[string]interface{}) *Callback {
	return C("fire/ProtectedFieldsValidator", Only(Create, Update), func(ctx *Context) error {
		// handle resource creation
		if ctx.Operation == Create {
			// check all fields
			for field, def := range pairs {
				// skip fields that have no default
				if def == NoDefault {
					continue
				}

				// check equality
				if !reflect.DeepEqual(ctx.Model.MustGet(field), def) {
					return Safe(errors.New("field " + field + " is protected"))
				}
			}
		}

		// handle resource updates
		if ctx.Operation == Update {
			// read the original
			original, err := ctx.Original()
			if err != nil {
				return err
			}

			// check all fields
			for field := range pairs {
				// check equality
				if !reflect.DeepEqual(ctx.Model.MustGet(field), original.MustGet(field)) {
					return Safe(errors.New("field " + field + " is protected"))
				}
			}
		}

		return nil
	})
}

// DependentResourcesValidator counts related documents and returns an error if
// some get found. This callback is meant to protect resources from breaking
// relations when requested to be deleted.
//
// Dependent resources are defined by passing pairs of models and fields that
// reference the current model.
//
//	fire.DependentResourcesValidator(map[string]string{
//		&Post{}: "Author",
//		&Comment{}: "Author",
//	})
//
func DependentResourcesValidator(pairs map[coal.Model]string) *Callback {
	return C("DependentResourcesValidator", Only(Delete), func(ctx *Context) error {
		// check all relations
		for model, field := range pairs {
			// prepare query
			query := bson.M{coal.F(model, field): ctx.Model.ID()}

			// count referencing documents
			ctx.Tracer.Push("mgo/Query.Count")
			ctx.Tracer.Tag("query", query)
			n, err := ctx.Store.DB().C(coal.C(model)).Find(query).Limit(1).Count()
			if err != nil {
				return err
			}
			ctx.Tracer.Pop()

			// return err of documents are found
			if n != 0 {
				return Safe(errors.New("resource has dependent resources"))
			}
		}

		// pass validation
		return nil
	})
}

// VerifyReferencesValidator makes sure all references in the document are
// existing by counting the references on the related collections.
//
// References are defined by passing pairs of fields and models who might be
// referenced by the current model:
//
//	fire.VerifyReferencesValidator(map[string]string{
//		"Post": &Post{},
//		"Author": &User{},
//	})
//
// The callbacks supports to-one, optional to-one and to-many relationships.
func VerifyReferencesValidator(pairs map[string]coal.Model) *Callback {
	return C("fire/VerifyReferencesValidator", Only(Create, Update), func(ctx *Context) error {
		// check all references
		for field, collection := range pairs {
			// read referenced id
			ref := ctx.Model.MustGet(field)

			// continue if reference is not set
			if oid, ok := ref.(*bson.ObjectId); ok && oid == nil {
				continue
			}

			// continue if slice is empty
			if ids, ok := ref.([]bson.ObjectId); ok && ids == nil {
				continue
			}

			// handle to-many relationships
			if ids, ok := ref.([]bson.ObjectId); ok {
				// prepare query
				query := bson.M{"_id": bson.M{"$in": ids}}

				// count entities in database
				ctx.Tracer.Push("mgo/Query.Count")
				ctx.Tracer.Tag("query", query)
				n, err := ctx.Store.DB().C(coal.C(collection)).Find(query).Count()
				if err != nil {
					return err
				}
				ctx.Tracer.Pop()

				// check for existence
				if n != len(ids) {
					return Safe(errors.New("missing references for field " + field))
				}

				continue
			}

			// handle to-one relationships

			// count entities in database
			ctx.Tracer.Push("mgo/Query.Count")
			ctx.Tracer.Tag("id", ref)
			n, err := ctx.Store.DB().C(coal.C(collection)).FindId(ref).Limit(1).Count()
			if err != nil {
				return err
			}
			ctx.Tracer.Pop()

			// check for existence
			if n != 1 {
				return Safe(errors.New("missing reference for field " + field))
			}
		}

		// pass validation
		return nil
	})
}

// RelationshipValidator makes sure all relationships of a model are correct and
// in place. It does so by combining a DependentResourcesValidator and a
// VerifyReferencesValidator based on the specified model and catalog.
func RelationshipValidator(model coal.Model, catalog *coal.Catalog, excludedFields ...string) *Callback {
	// prepare lists
	dependentResources := make(map[coal.Model]string)
	references := make(map[string]coal.Model)

	// iterate through all fields
	for _, field := range coal.Init(model).Meta().Fields {
		// exclude field if requested
		if Contains(excludedFields, field.Name) {
			continue
		}

		// handle has-one and has-many relationships
		if field.HasOne || field.HasMany {
			// get related model
			relatedModel := catalog.Find(field.RelType)
			if relatedModel == nil {
				panic("fire: missing model in catalog: " + field.RelType)
			}

			// get related bson field
			bsonField := ""
			for _, relatedField := range relatedModel.Meta().Fields {
				if relatedField.RelName == field.RelInverse {
					bsonField = relatedField.Name
				}
			}
			if bsonField == "" {
				panic("fire: missing field for inverse relationship: " + field.RelInverse)
			}

			// add relationship
			dependentResources[relatedModel] = bsonField
		}

		// handle to-one and to-many relationships
		if field.ToOne || field.ToMany {
			// get related model
			relatedModel := catalog.Find(field.RelType)
			if relatedModel == nil {
				panic("fire: missing model in catalog: " + field.RelType)
			}

			// add relationship
			references[field.Name] = relatedModel
		}
	}

	// create callbacks
	cb1 := DependentResourcesValidator(dependentResources)
	cb2 := VerifyReferencesValidator(references)

	return C("RelationshipValidator", func(ctx *Context) bool {
		return cb1.Matcher(ctx) || cb2.Matcher(ctx)
	}, func(ctx *Context) error {
		// run dependent resources validator
		if cb1.Matcher(ctx) {
			err := cb1.Handler(ctx)
			if err != nil {
				return err
			}
		}

		// run dependent resources validator
		if cb2.Matcher(ctx) {
			err := cb2.Handler(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// MatchingReferencesValidator compares the model with one related model or all
// related models and checks if the specified references are shared exactly.
//
// The target is defined by passing the reference on the current model and the
// target model. The matcher is defined by passing pairs of fields on the current
// and target model:
//
//	fire.MatchingReferencesValidator("Blog", &Blog{}, map[string]string{
//		"Owner": "Owner",
//	})
//
// To-many, optional to-many and has-many relationships are supported both for
// the initial reference and in the matchers.
func MatchingReferencesValidator(reference string, target coal.Model, matcher map[string]string) *Callback {
	return C("fire/MatchingReferencesValidator", Only(Create, Update), func(ctx *Context) error {
		// prepare ids
		var ids []bson.ObjectId

		// get reference
		ref := ctx.Model.MustGet(reference)

		// handle to-one reference
		if id, ok := ref.(bson.ObjectId); ok {
			ids = []bson.ObjectId{id}
		}

		// handle optional to-one reference
		if oid, ok := ref.(*bson.ObjectId); ok {
			// return immediately if not set
			if oid == nil {
				return nil
			}

			// set id
			ids = []bson.ObjectId{*oid}
		}

		// handle to-many reference
		if list, ok := ref.([]bson.ObjectId); ok {
			// return immediately if empty
			if len(list) == 0 {
				return nil
			}

			// set list
			ids = list
		}

		// ensure list is unique
		ids = coal.Unique(ids)

		// prepare query
		query := bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		}

		// add matchers
		for sourceField, targetField := range matcher {
			query[coal.F(target, targetField)] = ctx.Model.MustGet(sourceField)
		}

		// find matching documents
		ctx.Tracer.Push("mgo/Query.Count")
		ctx.Tracer.Tag("query", query)
		n, err := ctx.Store.DB().C(coal.C(target)).Find(query).Count()
		if err != nil {
			return err
		}
		ctx.Tracer.Pop()

		// return error if a document is missing (does not match)
		if n != len(ids) {
			return Safe(errors.New("references do not match"))
		}

		return nil
	})
}

// NoZero indicates that the zero value check should be skipped.
const NoZero noZero = iota

type noZero int

// UniqueFieldValidator ensures that the specified field of the current model will
// remain unique among the specified filters. If the value matches the provided
// zero value the check is skipped.
//
//	fire.UniqueFieldValidator("Name", "", "Creator")
//
// The special NoZero value can be provided to skip the zero check.
func UniqueFieldValidator(field string, zero interface{}, filters ...string) *Callback {
	return C("fire/UniqueFieldValidator", Only(Create, Update), func(ctx *Context) error {
		// check if field has changed
		if ctx.Operation == Update {
			// get original model
			original, err := ctx.Original()
			if err != nil {
				return err
			}

			// return if field has not been changed
			if reflect.DeepEqual(ctx.Model.MustGet(field), original.MustGet(field)) {
				return nil
			}
		}

		// get value
		value := ctx.Model.MustGet(field)

		// return if value is the zero value
		if value != NoZero && reflect.DeepEqual(value, zero) {
			return nil
		}

		// prepare query
		query := bson.M{
			coal.F(ctx.Model, field): value,
		}

		// add filters
		for _, field := range filters {
			query[coal.F(ctx.Model, field)] = ctx.Model.MustGet(field)
		}

		// count
		ctx.Tracer.Push("mgo/Query.Count")
		ctx.Tracer.Tag("query", query)
		n, err := ctx.Store.C(ctx.Model).Find(query).Limit(1).Count()
		if err != nil {
			return err
		} else if n != 0 {
			return Safe(fmt.Errorf("attribute %s is not unique", field))
		}
		ctx.Tracer.Pop()

		return nil
	})
}
