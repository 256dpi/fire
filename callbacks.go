package fire

import (
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// ErrAccessDenied can be returned by any callback to deny access.
var ErrAccessDenied = xo.BW(jsonapi.ErrorFromStatus(http.StatusUnauthorized, "access denied"))

// BasicAuthorizer authorizes requests based on a simple credentials list.
func BasicAuthorizer(credentials map[string]string) *Callback {
	return C("fire/BasicAuthorizer", All(), func(ctx *Context) error {
		// check for credentials
		user, password, ok := ctx.HTTPRequest.BasicAuth()
		if !ok {
			return ErrAccessDenied.Wrap()
		}

		// check if credentials match
		if val, ok := credentials[user]; !ok || val != password {
			return ErrAccessDenied.Wrap()
		}

		return nil
	})
}

// TimestampModifier will set timestamp fields on create and update operations.
// The fields are inferred from the model using the "fire-created-timestamp" and
// "fire-updated-timestamp" flags. Missing created timestamps are retroactively
// set using the timestamp encoded in the model id.
func TimestampModifier() *Callback {
	return C("fire/TimestampModifier", Only(Create|Update), func(ctx *Context) error {
		// get time
		now := time.Now()

		// get timestamp fields
		ctf := coal.L(ctx.Model, "fire-created-timestamp", false)
		utf := coal.L(ctx.Model, "fire-updated-timestamp", false)

		// set created timestamp on creation and set missing create timestamps
		// to the timestamp inferred from the model id
		if ctf != "" {
			if ctx.Operation == Create {
				stick.MustSet(ctx.Model, ctf, now)
			} else if t := stick.MustGet(ctx.Model, ctf).(time.Time); t.IsZero() {
				stick.MustSet(ctx.Model, ctf, ctx.Model.ID().Timestamp())
			}
		}

		// always set updated timestamp
		if utf != "" {
			stick.MustSet(ctx.Model, utf, now)
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
	return C("fire/ProtectedFieldsValidator", Only(Create|Update), func(ctx *Context) error {
		// handle resource creation
		if ctx.Operation == Create {
			// check all fields
			for field, def := range pairs {
				// skip fields that have no default
				if def == NoDefault {
					continue
				}

				// check equality
				if !reflect.DeepEqual(stick.MustGet(ctx.Model, field), def) {
					return xo.SF("field " + field + " is protected")
				}
			}
		}

		// handle resource updates
		if ctx.Operation == Update {
			// check all fields
			for field := range pairs {
				if ctx.Modified(field) {
					return xo.SF("field " + field + " is protected")
				}
			}
		}

		return nil
	})
}

// DependentResourcesValidator counts related documents and returns an error if
// some are found. This callback is meant to protect resources from breaking
// relations when requested to be deleted.
//
// Dependent resources are defined by passing pairs of models and fields that
// reference the current model.
//
//	fire.DependentResourcesValidator(map[coal.Model]string{
//		&Post{}:    "Author",
//		&Comment{}: "Author",
//	})
//
// The callback supports models that use the soft delete mechanism.
func DependentResourcesValidator(pairs map[coal.Model]string) *Callback {
	return C("fire/DependentResourcesValidator", Only(Delete), func(ctx *Context) error {
		// check all relations
		for model, field := range pairs {
			// prepare query
			query := bson.M{
				field: ctx.Model.ID(),
			}

			// exclude soft deleted documents if supported
			if sdf := coal.L(model, "fire-soft-delete", false); sdf != "" {
				query[sdf] = nil
			}

			// count referencing documents
			count, err := ctx.Store.M(model).Count(ctx, query, 0, 1, false)
			if err != nil {
				return err
			}

			// return error if documents are found
			if count != 0 {
				return xo.SF("resource has dependent resources")
			}
		}

		// pass validation
		return nil
	})
}

// ReferencedResourcesValidator makes sure all references in the document are
// existing by counting the referenced documents.
//
// References are defined by passing pairs of fields and models which are
// referenced by the current model:
//
//	fire.ReferencedResourcesValidator(map[string]coal.Model{
//		"Post":   &Post{},
//		"Author": &User{},
//	})
//
// The callbacks supports to-one, optional to-one and to-many relationships.
func ReferencedResourcesValidator(pairs map[string]coal.Model) *Callback {
	return C("fire/ReferencedResourcesValidator", Only(Create|Update), func(ctx *Context) error {
		// check all references
		for field, collection := range pairs {
			// read referenced id
			ref := stick.MustGet(ctx.Model, field)

			// continue if reference is not set
			if oid, ok := ref.(*coal.ID); ok && oid == nil {
				continue
			}

			// continue if slice is empty
			if ids, ok := ref.([]coal.ID); ok && ids == nil {
				continue
			}

			// handle to-many relationships
			if ids, ok := ref.([]coal.ID); ok {
				// prepare query
				query := bson.M{
					"_id": bson.M{
						"$in": ids,
					},
				}

				// count entities in database
				count, err := ctx.Store.M(collection).Count(ctx, query, 0, 0, false)
				if err != nil {
					return err
				}

				// check for existence
				if int(count) != len(ids) {
					return xo.SF("missing references for field " + field)
				}

				continue
			}

			// handle to-one relationships

			// count entities in database
			count, err := ctx.Store.M(collection).Count(ctx, bson.M{
				"_id": ref,
			}, 0, 1, false)
			if err != nil {
				return err
			}

			// check for existence
			if count != 1 {
				return xo.SF("missing reference for field " + field)
			}
		}

		// pass validation
		return nil
	})
}

// RelationshipValidator makes sure all relationships of a model are correct and
// in place. It does so by combining a DependentResourcesValidator and a
// ReferencedResourcesValidator based on the specified model and catalog.
func RelationshipValidator(model coal.Model, catalog *coal.Catalog, exclude ...string) *Callback {
	// prepare lists
	resources := make(map[coal.Model]string)
	references := make(map[string]coal.Model)

	// iterate through all fields
	for _, field := range coal.GetMeta(model).Relationships {
		// continue if relationship is excluded
		if stick.Contains(exclude, field.Name) {
			continue
		}

		// handle has-one and has-many relationships
		if field.HasOne || field.HasMany {
			// get related model
			relatedModel := catalog.Find(field.RelType)
			if relatedModel == nil {
				panic(fmt.Sprintf(`fire: missing model in catalog: "%s"`, field.RelType))
			}

			// get related field
			relatedField := ""
			for _, relationship := range coal.GetMeta(relatedModel).Relationships {
				if relationship.RelName == field.RelInverse {
					relatedField = relationship.Name
				}
			}
			if relatedField == "" {
				panic(fmt.Sprintf(`fire: missing field for inverse relationship: "%s"`, field.RelInverse))
			}

			// add resource
			resources[relatedModel] = relatedField
		}

		// handle to-one and to-many relationships
		if field.ToOne || field.ToMany {
			// get related model
			relatedModel := catalog.Find(field.RelType)
			if relatedModel == nil {
				panic(fmt.Sprintf(`fire: missing model in catalog: "%s"`, field.RelType))
			}

			// add reference
			references[field.Name] = relatedModel
		}
	}

	// create callbacks
	drv := DependentResourcesValidator(resources)
	rrv := ReferencedResourcesValidator(references)

	// combine callbacks
	cb := Combine("fire/RelationshipValidator", drv, rrv)

	return cb
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
	return C("fire/MatchingReferencesValidator", Only(Create|Update), func(ctx *Context) error {
		// prepare ids
		var ids []coal.ID

		// get reference
		ref := stick.MustGet(ctx.Model, reference)

		// handle to-one reference
		if id, ok := ref.(coal.ID); ok {
			ids = []coal.ID{id}
		}

		// handle optional to-one reference
		if oid, ok := ref.(*coal.ID); ok {
			// return immediately if not set
			if oid == nil {
				return nil
			}

			// set id
			ids = []coal.ID{*oid}
		}

		// handle to-many reference
		if list, ok := ref.([]coal.ID); ok {
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
			query[targetField] = stick.MustGet(ctx.Model, sourceField)
		}

		// find matching documents
		count, err := ctx.Store.M(target).Count(ctx, query, 0, 0, false)
		if err != nil {
			return err
		}

		// return error if a document is missing (does not match)
		if int(count) != len(ids) {
			return xo.SF("references do not match")
		}

		return nil
	})
}
