package fire

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/256dpi/fire/coal"

	"gopkg.in/mgo.v2/bson"
)

// C is a short-hand function to construct a callback. It will also add tracing
// code around the execution of the callback.
func C(name string, m Matcher, h Handler) *Callback {
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
//
// If a returned error is wrapped using Fatal, processing stops immediately and
// the error is logged.
type Handler func(*Context) error

// The matcher should return true if the callback should be run for the
// presented action.
type Matcher func(*Context) bool

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
	Matcher Matcher
	Handler Handler
}

// Combine will return a callback that runs all the specified callbacks in order
// until an error is returned.
func Combine(cbs ...*Callback) *Callback {
	return C("fire/Combine", nil, func(ctx *Context) error {
		// run all callbacks
		for _, cb := range cbs {
			err := cb.Handler(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

// BasicAuthorizer authorizes requests based on a simple credentials list.
func BasicAuthorizer(credentials map[string]string) *Callback {
	return C("fire/BasicAuthorizer", nil, func(ctx *Context) error {
		// check for credentials
		user, password, ok := ctx.HTTPRequest.BasicAuth()
		if !ok {
			return errors.New("access denied")
		}

		// check if credentials match
		if val, ok := credentials[user]; !ok || val != password {
			return errors.New("access denied")
		}

		return nil
	})
}

// ModelValidator performs a validation of the model using the Validate
// function.
func ModelValidator() *Callback {
	return C("fire/ModelValidator", Only(Create, Update), func(ctx *Context) error {
		// TODO: Add error source pointers.
		return coal.Validate(ctx.Model)
	})
}

type noDefault int

// NoDefault marks the specified field to have no default that needs to be
// enforced while executing the ProtectedFieldsValidator.
const NoDefault noDefault = iota

// ProtectedFieldsValidator compares protected attributes against their
// default (during Create) or stored value (during Update) and returns an error
// if they have been changed.
//
// Attributes are defined by passing pairs of fields and default values:
//
//	ProtectedFieldsValidator(map[string]interface{}{
//		F(&Post{}, "Title"): NoDefault, // can only be set during Create
//		F(&Post{}, "Link"):  "",        // is fixed and cannot be changed
//	})
//
// The special NoDefault value can be provided to skip the default enforcement
// on Create.
func ProtectedFieldsValidator(fields map[string]interface{}) *Callback {
	return C("fire/ProtectedFieldsValidator", Only(Create, Update), func(ctx *Context) error {
		// handle resource creation
		if ctx.Operation == Create {
			// check all fields
			for field, def := range fields {
				// skip fields that have no default
				if def == NoDefault {
					continue
				}

				// check equality
				if !reflect.DeepEqual(ctx.Model.MustGet(field), def) {
					return errors.New("field " + field + " is protected")
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
			for field := range fields {
				// check equality
				if !reflect.DeepEqual(ctx.Model.MustGet(field), original.MustGet(field)) {
					return errors.New("field " + field + " is protected")
				}
			}
		}

		return nil
	})
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
//		C(&Comment{}): F(&Comment{}, "Author"),
//	})
//
func DependentResourcesValidator(resources map[string]string) *Callback {
	return C("DependentResourcesValidator", Only(Delete), func(ctx *Context) error {
		// check all relations
		for coll, field := range resources {
			// prepare query
			query := bson.M{field: ctx.Model.ID()}

			// count referencing documents
			ctx.Tracer.Push("mgo/Query.Count")
			ctx.Tracer.Tag("query", query)
			n, err := ctx.Store.DB().C(coll).Find(query).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}
			ctx.Tracer.Pop()

			// return err of documents are found
			if n != 0 {
				return errors.New("resource has dependent resources")
			}
		}

		// pass validation
		return nil
	})
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
// The callbacks supports to-one, optional to-one and to-many relationships.
func VerifyReferencesValidator(references map[string]string) *Callback {
	return C("fire/VerifyReferencesValidator", Only(Create, Update), func(ctx *Context) error {
		// check all references
		for field, collection := range references {
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
				n, err := ctx.Store.DB().C(collection).Find(query).Count()
				if err != nil {
					return Fatal(err)
				}
				ctx.Tracer.Pop()

				// check for existence
				if n != len(ids) {
					return errors.New("missing references for field " + field)
				}

				continue
			}

			// handle to-one relationships

			// count entities in database
			ctx.Tracer.Push("mgo/Query.Count")
			ctx.Tracer.Tag("id", ref)
			n, err := ctx.Store.DB().C(collection).FindId(ref).Limit(1).Count()
			if err != nil {
				return Fatal(err)
			}
			ctx.Tracer.Pop()

			// check for existence
			if n != 1 {
				return errors.New("missing reference for field " + field)
			}
		}

		// pass validation
		return nil
	})
}

// RelationshipValidator makes sure all relationships of a model are correct and
// in place. It does so by creating a DependentResourcesValidator and a
// VerifyReferencesValidator based on the specified model and catalog.
func RelationshipValidator(model coal.Model, catalog *coal.Catalog, excludedFields ...string) *Callback {
	// prepare lists
	dependentResources := make(map[string]string)
	references := make(map[string]string)

	// iterate through all fields
	for _, field := range coal.Init(model).Meta().Fields {
		// exclude field if requested
		if stringInList(field.Name, excludedFields) {
			continue
		}

		// handle has-one and has-many relationships
		if field.HasOne || field.HasMany {
			// get related model
			relatedModel := catalog.Find(field.RelType)
			if relatedModel == nil {
				panic("fire: missing model in catalog: " + field.RelType)
			}

			// get collection
			collection := relatedModel.Meta().Collection

			// get related bson field
			bsonField := ""
			for _, relatedField := range relatedModel.Meta().Fields {
				if relatedField.RelName == field.RelInverse {
					bsonField = relatedField.BSONName
				}
			}
			if bsonField == "" {
				panic("fire: missing field for inverse relationship: " + field.RelInverse)
			}

			// add relationship
			dependentResources[collection] = bsonField
		}

		// handle to-one and to-many relationships
		if field.ToOne || field.ToMany {
			// get related model
			relatedModel := catalog.Find(field.RelType)
			if relatedModel == nil {
				panic("fire: missing model in catalog: " + field.RelType)
			}

			// add relationship
			references[field.BSONName] = relatedModel.Meta().Collection
		}
	}

	// create callbacks
	cb1 := DependentResourcesValidator(dependentResources)
	cb2 := VerifyReferencesValidator(references)

	// create a combined callback
	cb := Combine(cb1, cb2)

	return cb
}

// MatchingReferencesValidator compares the model with one related model or all
// related models and checks if the specified references are exactly shared.
//
// The target model is defined by passing its collection and the referencing
// field on the current model. The matcher is defined by passing pairs of
// database fields on the target and current model:
//
//	MatchingReferencesValidator(C(&Blog{}), F(&Post{}, "Blog"), map[string]string{
//		F(&Blog{}, "Owner"): F(&Post{}, "Owner"),
//	})
//
// To-many, optional to-many and has-many relationships are supported both for
// the initial reference and in the matchers.
func MatchingReferencesValidator(collection, reference string, matcher map[string]string) *Callback {
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

		// add matchers as-is
		for targetField, modelField := range matcher {
			query[targetField] = ctx.Model.MustGet(modelField)
		}

		// find matching documents
		ctx.Tracer.Push("mgo/Query.Count")
		ctx.Tracer.Tag("query", query)
		n, err := ctx.Store.DB().C(collection).Find(query).Count()
		if err != nil {
			return Fatal(err)
		}
		ctx.Tracer.Pop()

		// return error if a document is missing (does not match)
		if n != len(ids) {
			return errors.New("references do not match")
		}

		return nil
	})
}

// UniqueAttributeValidator ensures that the specified attribute of the
// controllers Model will remain unique among the specified filters.
//
// The unique attribute is defines as the first argument. Filters are defined
// by passing a list of database fields:
//
//	UniqueAttributeValidator(F(&Blog{}, "Name"), F(&Blog{}, "Creator"))
//
func UniqueAttributeValidator(uniqueAttribute string, filters ...string) *Callback {
	return C("fire/UniqueAttributeValidator", Only(Create, Update), func(ctx *Context) error {
		// check if field has changed
		if ctx.Operation == Update {
			// get original model
			original, err := ctx.Original()
			if err != nil {
				return err
			}

			// return if field has not been changed
			if reflect.DeepEqual(ctx.Model.MustGet(uniqueAttribute), original.MustGet(uniqueAttribute)) {
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
		ctx.Tracer.Push("mgo/Query.Count")
		ctx.Tracer.Tag("query", query)
		n, err := ctx.Store.C(ctx.Model).Find(query).Limit(1).Count()
		if err != nil {
			return Fatal(err)
		} else if n != 0 {
			return fmt.Errorf("attribute %s is not unique", uniqueAttribute)
		}
		ctx.Tracer.Pop()

		return nil
	})
}
