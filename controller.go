package fire

import (
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
	"github.com/256dpi/stack"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
)

// L is a short-hand type to create a list of callbacks.
type L []*Callback

// M is a short-hand type to create a map of actions.
type M map[string]*Action

// An Action defines a collection or resource action.
type Action struct {
	// The allowed methods for this action.
	Methods []string

	// The callback for this action.
	Callback *Callback

	// BodyLimit defines the maximum allowed size of the request body. It
	// defaults to 8M if set to zero. The DataSize helper can be used to set
	// the value.
	BodyLimit uint64
}

// A Controller provides a JSON API based interface to a model.
//
// Note: A controller must not be modified after being added to a group.
type Controller struct {
	// The model that this controller should provide (e.g. &Foo{}).
	Model coal.Model

	// The store that is used to retrieve and persist the model.
	Store *coal.Store

	// Filters is a list of fields that are filterable. Only fields that are
	// exposed and indexed should be made filterable.
	Filters []string

	// Sorters is a list of fields that are sortable. Only fields that are
	// exposed and indexed should be made sortable.
	Sorters []string

	// Authorizers authorize the requested operation on the requested resource
	// and are run before any models are loaded from the DB. Returned errors
	// will cause the abortion of the request with an unauthorized status by
	// default.
	//
	// The callbacks are expected to return an error if the requester should be
	// informed about him being unauthorized to access the resource, or add
	// filters to the context to only return accessible resources. The later
	// improves privacy as a protected resource would appear as being not found.
	Authorizers []*Callback

	// Validators are run to validate Create, Update and Delete operations
	// after the models are loaded and the changed attributes have been assigned
	// during an Update. Returned errors will cause the abortion of the request
	// with a bad request status by default.
	//
	// The callbacks are expected to validate the model being created, updated or
	// deleted and return errors if the presented attributes or relationships
	// are invalid or do not comply with the stated requirements. Necessary
	// authorization checks should be repeated and now also include the model's
	// attributes and relationships.
	Validators []*Callback

	// Decorators are run after the models or model have been loaded from the
	// database for List and Find operations or the model has been saved or
	// updated for Create and Update operations. Returned errors will cause the
	// abortion of the request with an Internal Server Error status by default.
	Decorators []*Callback

	// Notifiers are run before the final response is written to the client
	// and provide a chance to modify the response and notify other systems
	// about the applied changes. Returned errors will cause the abortion of the
	// request with an Internal Server Error status by default.
	Notifiers []*Callback

	// NoList can be set to true if the resource is only listed through
	// relationships from other resources. This is useful for resources like
	// comments that should never be listed alone.
	NoList bool

	// ListLimit can be set to a value higher than 1 to enforce paginated
	// responses and restrain the page size to be within one and the limit.
	//
	// Note: Fire uses the "page[number]" and "page[size]" query parameters for
	// pagination.
	ListLimit uint64

	// DocumentLimit defines the maximum allowed size of an incoming document.
	// It defaults to 8M if set to zero. The DataSize helper can be used to
	// set the value.
	DocumentLimit uint64

	// CollectionActions and ResourceActions are custom actions that are run
	// on the collection (e.g. "posts/delete-cache") or resource (e.g.
	// "posts/1/recover-password"). The request context is forwarded to
	// the specified callback after running the authorizers. No validators and
	// notifiers are run for the request.
	CollectionActions map[string]*Action
	ResourceActions   map[string]*Action

	// SoftProtection will not raise an error if a non-writable field is set
	// during a Create or Update operation. Frameworks like Ember.js just
	// serialize the complete state of a model and thus might send attributes
	// and relationships that are not writable.
	SoftProtection bool

	// SoftDeleteField can be set to a model field of type *time.Time that will
	// be used to soft delete records. The controller will set this field to the
	// time of deletion instead of removing the document while the API for the
	// consumer does not change. It is advised to create a TTL index to delete
	// the documents after a timeout.
	SoftDeleteField string

	parser jsonapi.Parser
}

func (c *Controller) prepare() {
	// initialize model
	coal.Init(c.Model)

	// prepare parser
	c.parser = jsonapi.Parser{
		CollectionActions: make(map[string][]string),
		ResourceActions:   make(map[string][]string),
	}

	// add collection actions
	for name, action := range c.CollectionActions {
		// check collision
		if name == "" || bson.IsObjectIdHex(name) {
			panic(fmt.Sprintf(`fire: invalid collection action "%s"`, name))
		}

		// set default body limit
		if action.BodyLimit == 0 {
			action.BodyLimit = DataSize("8M")
		}

		// add action to parser
		c.parser.CollectionActions[name] = action.Methods
	}

	// add resource actions
	for name, action := range c.ResourceActions {
		// check collision
		if name == "" || name == "relationships" || c.Model.Meta().Relationships[name] != nil {
			panic(fmt.Sprintf(`fire: invalid resource action "%s"`, name))
		}

		// set default body limit
		if action.BodyLimit == 0 {
			action.BodyLimit = DataSize("8M")
		}

		// add action to parser
		c.parser.ResourceActions[name] = action.Methods
	}

	// set default document limit
	if c.DocumentLimit == 0 {
		c.DocumentLimit = DataSize("8M")
	}

	// check soft delete field
	if c.SoftDeleteField != "" {
		// get field
		field, ok := c.Model.Meta().Fields[c.SoftDeleteField]
		if !ok {
			panic(fmt.Sprintf(`fire: missing soft delete field "%s" for model "%s"`, c.SoftDeleteField, c.Model.Meta().Name))
		}

		// check field type
		if field.Type.String() != "*time.Time" {
			panic(fmt.Sprintf(`fire: soft delete field "%s" for model "%s" is not of type "*time.Time"`, c.SoftDeleteField, c.Model.Meta().Name))
		}
	}
}

func (c *Controller) generalHandler(prefix string, ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.generalHandler")

	// parse incoming JSON-API request
	c.parser.Prefix = prefix
	req, err := c.parser.ParseRequest(ctx.HTTPRequest)
	stack.AbortIf(err)

	// set request
	ctx.JSONAPIRequest = req

	// handle no list setting
	if req.Intent == jsonapi.ListResources && c.NoList {
		stack.Abort(jsonapi.ErrorFromStatus(
			http.StatusMethodNotAllowed,
			"listing is disabled for this resource",
		))
	}

	// parse document if expected
	var doc *jsonapi.Document
	if req.Intent.DocumentExpected() {
		// limit request body size
		LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, int64(c.DocumentLimit))

		// parse document and respect document limit
		doc, err = jsonapi.ParseDocument(ctx.HTTPRequest.Body)
		stack.AbortIf(err)
	}

	// validate id if present
	if req.ResourceID != "" && !bson.IsObjectIdHex(req.ResourceID) {
		stack.Abort(jsonapi.BadRequest("invalid resource id"))
	}

	// prepare context
	ctx.Selector = bson.M{}
	ctx.Filters = []bson.M{}
	ctx.ReadableFields = c.initialFields(c.Model, ctx.JSONAPIRequest)
	ctx.WritableFields = c.initialFields(c.Model, nil)

	// copy store
	store := c.Store.Copy()
	defer store.Close()

	// set store
	ctx.Store = store

	// call specific handlers
	switch req.Intent {
	case jsonapi.ListResources:
		c.listResources(ctx)
	case jsonapi.FindResource:
		c.findResource(ctx)
	case jsonapi.CreateResource:
		c.createResource(ctx, doc)
	case jsonapi.UpdateResource:
		c.updateResource(ctx, doc)
	case jsonapi.DeleteResource:
		c.deleteResource(ctx)
	case jsonapi.GetRelatedResources:
		c.getRelatedResources(ctx)
	case jsonapi.GetRelationship:
		c.getRelationship(ctx)
	case jsonapi.SetRelationship:
		c.setRelationship(ctx, doc)
	case jsonapi.AppendToRelationship:
		c.appendToRelationship(ctx, doc)
	case jsonapi.RemoveFromRelationship:
		c.removeFromRelationship(ctx, doc)
	case jsonapi.CollectionAction:
		c.handleCollectionAction(ctx)
	case jsonapi.ResourceAction:
		c.handleResourceAction(ctx)
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) listResources(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.listResources")

	// set operation
	ctx.Operation = List

	// load models
	c.loadModels(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			Many: c.resourcesForModels(ctx, ctx.Models),
		},
		Links: c.listLinks(ctx.JSONAPIRequest.Self(), ctx),
	}

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) findResource(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.findResource")

	// set operation
	ctx.Operation = Find

	// load model
	c.loadModel(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			One: c.resourceForModel(ctx, ctx.Model),
		},
		Links: &jsonapi.DocumentLinks{
			Self: ctx.JSONAPIRequest.Self(),
		},
	}

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) createResource(ctx *Context, doc *jsonapi.Document) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.createResource")

	// set operation
	ctx.Operation = Create

	// basic input data check
	if doc.Data == nil || doc.Data.One == nil {
		stack.Abort(jsonapi.BadRequest("missing document"))
	}

	// check resource type
	if doc.Data.One.Type != ctx.JSONAPIRequest.ResourceType {
		stack.Abort(jsonapi.BadRequest("resource type mismatch"))
	}

	// check id
	if doc.Data.One.ID != "" {
		stack.Abort(jsonapi.BadRequest("unnecessary resource id"))
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// create new model
	ctx.Model = c.Model.Meta().Make()

	// assign attributes
	c.assignData(ctx, doc.Data.One)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// insert model
	ctx.Tracer.Push("mgo/Collection.Insert")
	ctx.Tracer.Tag("model", ctx.Model)
	stack.AbortIf(ctx.Store.C(ctx.Model).Insert(ctx.Model))
	ctx.Tracer.Pop()

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			One: c.resourceForModel(ctx, ctx.Model),
		},
		Links: &jsonapi.DocumentLinks{
			Self: ctx.JSONAPIRequest.Self() + "/" + ctx.Model.ID().Hex(),
		},
	}

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusCreated, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) updateResource(ctx *Context, doc *jsonapi.Document) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.updateResource")

	// set operation
	ctx.Operation = Update

	// basic input data check
	if doc.Data == nil || doc.Data.One == nil {
		stack.Abort(jsonapi.BadRequest("missing document"))
	}

	// check resource type
	if doc.Data.One.Type != ctx.JSONAPIRequest.ResourceType {
		stack.Abort(jsonapi.BadRequest("resource type mismatch"))
	}

	// check id
	if doc.Data.One.ID != ctx.JSONAPIRequest.ResourceID {
		stack.Abort(jsonapi.BadRequest("resource id mismatch"))
	}

	// load model
	c.loadModel(ctx)

	// assign attributes
	c.assignData(ctx, doc.Data.One)

	// save model
	c.updateModel(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			One: c.resourceForModel(ctx, ctx.Model),
		},
		Links: &jsonapi.DocumentLinks{
			Self: ctx.JSONAPIRequest.Self(),
		},
	}

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) deleteResource(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.deleteResource")

	// set operation
	ctx.Operation = Delete

	// load model
	c.loadModel(ctx)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// soft delete or remove model
	if c.SoftDeleteField != "" {
		// soft delete model
		ctx.Tracer.Push("mgo/Collection.UpdateId")
		ctx.Tracer.Tag("model", ctx.Model)
		stack.AbortIf(ctx.Store.C(c.Model).UpdateId(ctx.Model.ID(), bson.M{
			"$set": bson.M{
				coal.F(c.Model, c.SoftDeleteField): time.Now(),
			},
		}))
		ctx.Tracer.Pop()
	} else {
		// remove model
		ctx.Tracer.Push("mgo/Collection.RemoveId")
		ctx.Tracer.Tag("model", ctx.Model)
		stack.AbortIf(ctx.Store.C(c.Model).RemoveId(ctx.Model.ID()))
		ctx.Tracer.Pop()
	}

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// set status
	ctx.ResponseWriter.WriteHeader(http.StatusNoContent)

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) getRelatedResources(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.getRelatedResources")

	// find relationship
	rel := c.Model.Meta().Relationships[ctx.JSONAPIRequest.RelatedResource]
	if rel == nil {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// set operation
	ctx.Operation = Find

	// load model
	c.loadModel(ctx)

	// check if relationship is readable
	if !Contains(ctx.ReadableFields, rel.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not readable"))
	}

	// get related controller
	rc := ctx.Group.controllers[rel.RelType]
	if rc == nil {
		stack.Abort(fmt.Errorf("missing related controller for %s", rel.RelType))
	}

	// copy context and request
	newCtx := &Context{
		Selector:       bson.M{},
		Filters:        []bson.M{},
		ReadableFields: rc.initialFields(rc.Model, ctx.JSONAPIRequest),
		WritableFields: rc.initialFields(rc.Model, nil),
		Store:          ctx.Store,
		JSONAPIRequest: &jsonapi.Request{
			Prefix:       ctx.JSONAPIRequest.Prefix,
			ResourceType: rel.RelType,
			Include:      ctx.JSONAPIRequest.Include,
			PageNumber:   ctx.JSONAPIRequest.PageNumber,
			PageSize:     ctx.JSONAPIRequest.PageSize,
			PageOffset:   ctx.JSONAPIRequest.PageOffset,
			PageLimit:    ctx.JSONAPIRequest.PageLimit,
			Sorting:      ctx.JSONAPIRequest.Sorting,
			Fields:       ctx.JSONAPIRequest.Fields,
			Filters:      ctx.JSONAPIRequest.Filters,
		},
		HTTPRequest:    ctx.HTTPRequest,
		ResponseWriter: ctx.ResponseWriter,
		Controller:     rc,
		Group:          ctx.Group,
		Tracer:         ctx.Tracer,
	}

	// finish to-one relationship
	if rel.ToOne {
		// prepare id
		var id string

		// lookup id of related resource
		if rel.Optional {
			oid := ctx.Model.MustGet(rel.Name).(*bson.ObjectId)
			if oid != nil {
				id = oid.Hex()
			}
		} else {
			id = ctx.Model.MustGet(rel.Name).(bson.ObjectId).Hex()
		}

		// tweak context
		newCtx.Operation = Find
		newCtx.JSONAPIRequest.Intent = jsonapi.FindResource
		newCtx.JSONAPIRequest.ResourceID = id

		// prepare response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				One: nil,
			},
			Links: &jsonapi.DocumentLinks{
				Self: ctx.JSONAPIRequest.Self(),
			},
		}

		// load model if id is present
		if id != "" {
			// load model
			rc.loadModel(newCtx)

			// run decorators
			c.runCallbacks(c.Decorators, newCtx, http.StatusInternalServerError)

			// set model
			newCtx.Response.Data.One = rc.resourceForModel(newCtx, newCtx.Model)
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish to-many relationship
	if rel.ToMany {
		// get ids from loaded model
		ids := ctx.Model.MustGet(rel.Name).([]bson.ObjectId)

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query
		newCtx.Selector["_id"] = bson.M{"$in": ids}

		// load related models
		rc.loadModels(newCtx)

		// run decorators
		c.runCallbacks(c.Decorators, newCtx, http.StatusInternalServerError)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				Many: rc.resourcesForModels(newCtx, newCtx.Models),
			},
			Links: rc.listLinks(ctx.JSONAPIRequest.Self(), newCtx),
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish has-one relationship
	if rel.HasOne {
		// find relationship
		relRel := rc.Model.Meta().Relationships[rel.RelInverse]
		if relRel == nil {
			stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", relRel.RelInverse))
		}

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query
		newCtx.Selector[relRel.BSONField] = ctx.Model.ID()

		// load related models
		rc.loadModels(newCtx)

		// check models
		if len(newCtx.Models) > 1 {
			stack.Abort(fmt.Errorf("has one relationship returned more than one result"))
		}

		// run decorators if found
		if len(newCtx.Models) == 1 {
			c.runCallbacks(c.Decorators, newCtx, http.StatusInternalServerError)
		}

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{},
			Links: &jsonapi.DocumentLinks{
				Self: ctx.JSONAPIRequest.Self(),
			},
		}

		// add if model is found
		if len(newCtx.Models) == 1 {
			newCtx.Response.Data.One = rc.resourceForModel(newCtx, newCtx.Models[0])
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish has-many relationship
	if rel.HasMany {
		// find relationship
		relRel := rc.Model.Meta().Relationships[rel.RelInverse]
		if relRel == nil {
			stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", relRel.RelInverse))
		}

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query (supports to-one and to-many relationships)
		newCtx.Selector[relRel.BSONField] = bson.M{
			"$in": []bson.ObjectId{ctx.Model.ID()},
		}

		// load related models
		rc.loadModels(newCtx)

		// run decorators
		c.runCallbacks(c.Decorators, newCtx, http.StatusInternalServerError)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				Many: rc.resourcesForModels(newCtx, newCtx.Models),
			},
			Links: rc.listLinks(ctx.JSONAPIRequest.Self(), newCtx),
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) getRelationship(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.getRelationship")

	// get relationship
	field := c.Model.Meta().Relationships[ctx.JSONAPIRequest.Relationship]
	if field == nil {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// set operation
	ctx.Operation = Find

	// load model
	c.loadModel(ctx)

	// check if relationship is readable
	if !Contains(ctx.ReadableFields, field.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not readable"))
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) setRelationship(ctx *Context, doc *jsonapi.Document) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.setRelationship")

	// get relationship
	rel := c.Model.Meta().Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || (!rel.ToOne && !rel.ToMany) {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// set operation
	ctx.Operation = Update

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !Contains(ctx.WritableFields, rel.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// assign relationship
	c.assignRelationship(ctx, ctx.JSONAPIRequest.Relationship, doc, rel)

	// save model
	c.updateModel(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) appendToRelationship(ctx *Context, doc *jsonapi.Document) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.appendToRelationship")

	// get relationship
	rel := c.Model.Meta().Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || !rel.ToMany {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// set operation
	ctx.Operation = Update

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !Contains(ctx.WritableFields, rel.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// process all references
	for _, ref := range doc.Data.Many {
		// check type
		if ref.Type != rel.RelType {
			stack.Abort(jsonapi.BadRequest("resource type mismatch"))
		}

		// return error for an invalid id
		if !bson.IsObjectIdHex(ref.ID) {
			stack.Abort(jsonapi.BadRequest("invalid relationship id"))
		}

		// get id
		refID := bson.ObjectIdHex(ref.ID)

		// get current ids
		ids := ctx.Model.MustGet(rel.Name).([]bson.ObjectId)

		// check if id is already present
		if coal.Contains(ids, refID) {
			continue
		}

		// add id
		ids = append(ids, refID)
		ctx.Model.MustSet(rel.Name, ids)
	}

	// save model
	c.updateModel(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) removeFromRelationship(ctx *Context, doc *jsonapi.Document) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.removeFromRelationship")

	// get relationship
	rel := c.Model.Meta().Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || !rel.ToMany {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// set operation
	ctx.Operation = Update

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !Contains(ctx.WritableFields, rel.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// process all references
	for _, ref := range doc.Data.Many {
		// check type
		if ref.Type != rel.RelType {
			stack.Abort(jsonapi.BadRequest("resource type mismatch"))
		}

		// return error for an invalid id
		if !bson.IsObjectIdHex(ref.ID) {
			stack.Abort(jsonapi.BadRequest("invalid relationship id"))
		}

		// get id
		refID := bson.ObjectIdHex(ref.ID)

		// prepare mark
		var pos = -1

		// get current ids
		ids := ctx.Model.MustGet(rel.Name).([]bson.ObjectId)

		// check if id is already present
		for i, id := range ids {
			if id == refID {
				pos = i
			}
		}

		// remove id if present
		if pos >= 0 {
			ids = append(ids[:pos], ids[pos+1:]...)
			ctx.Model.MustSet(rel.Name, ids)
		}
	}

	// save model
	c.updateModel(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, ctx.Response))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) handleCollectionAction(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.handleCollectionAction")

	// set operation
	ctx.Operation = CollectionAction

	// get callback
	action, ok := c.CollectionActions[ctx.JSONAPIRequest.CollectionAction]
	if !ok {
		stack.Abort(fmt.Errorf("missing collection action callback"))
	}

	// limit request body size
	LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, int64(action.BodyLimit))

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// run callback
	c.runAction(action, ctx, http.StatusBadRequest)

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) handleResourceAction(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.handleResourceAction")

	// set operation
	ctx.Operation = ResourceAction

	// get callback
	action, ok := c.ResourceActions[ctx.JSONAPIRequest.ResourceAction]
	if !ok {
		stack.Abort(fmt.Errorf("missing resource action callback"))
	}

	// limit request body size
	LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, int64(action.BodyLimit))

	// load model
	c.loadModel(ctx)

	// run callback
	c.runAction(action, ctx, http.StatusBadRequest)

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) initialFields(model coal.Model, r *jsonapi.Request) []string {
	// prepare list
	list := make([]string, 0, len(model.Meta().Attributes)+len(model.Meta().Relationships))

	// add attributes
	for _, f := range model.Meta().Attributes {
		list = append(list, f.Name)
	}

	// add relationships
	for _, f := range model.Meta().Relationships {
		list = append(list, f.Name)
	}

	// check if a field whitelist has been provided
	if r != nil && len(r.Fields[model.Meta().PluralName]) > 0 {
		// convert requested fields list
		var requested []string
		for _, field := range r.Fields[model.Meta().PluralName] {
			// add attribute
			if f := model.Meta().Attributes[field]; f != nil {
				requested = append(requested, f.Name)
				continue
			}

			// add relationship
			if f := model.Meta().Relationships[field]; f != nil {
				requested = append(requested, f.Name)
				continue
			}

			// raise error
			stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid sparse field "%s"`, field)))
		}

		// whitelist requested fields
		list = Intersect(requested, list)
	}

	return list
}

func (c *Controller) loadModel(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.loadModel")

	// set selector query
	ctx.Selector["_id"] = bson.ObjectIdHex(ctx.JSONAPIRequest.ResourceID)

	// filter out deleted records if configured
	if c.SoftDeleteField != "" {
		ctx.Selector[coal.F(c.Model, c.SoftDeleteField)] = nil
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// prepare object
	obj := c.Model.Meta().Make()

	// query db
	ctx.Tracer.Push("mgo/Query.One")
	ctx.Tracer.Tag("query", ctx.Query())
	err := ctx.Store.C(c.Model).Find(ctx.Query()).One(obj)
	if err == mgo.ErrNotFound {
		stack.Abort(jsonapi.NotFound("resource not found"))
	}
	stack.AbortIf(err)
	ctx.Tracer.Pop()

	// initialize and set model
	ctx.Model = coal.Init(obj.(coal.Model))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) loadModels(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.loadModels")

	// filter out deleted records if configured
	if c.SoftDeleteField != "" {
		ctx.Selector[coal.F(c.Model, c.SoftDeleteField)] = nil
	}

	// add filters
	for name, values := range ctx.JSONAPIRequest.Filters {
		// handle attributes filter
		if field := c.Model.Meta().Attributes[name]; field != nil {
			// check whitelist
			if !Contains(c.Filters, field.Name) {
				stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
			}

			// handle boolean values
			if field.Kind == reflect.Bool && len(values) == 1 {
				ctx.Filters = append(ctx.Filters, bson.M{field.BSONField: values[0] == "true"})
				continue
			}

			// handle string values
			ctx.Filters = append(ctx.Filters, bson.M{field.BSONField: bson.M{"$in": values}})
			continue
		}

		// handle relationship filters
		if field := c.Model.Meta().Relationships[name]; field != nil {
			// check whitelist
			if !field.ToOne && !field.ToMany || !Contains(c.Filters, field.Name) {
				stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
			}

			// convert to object ids
			var ids []bson.ObjectId
			for _, str := range values {
				if !bson.IsObjectIdHex(str) {
					stack.Abort(jsonapi.BadRequest("relationship filter value is not an object id"))
				}
				ids = append(ids, bson.ObjectIdHex(str))
			}

			// set relationship filter
			ctx.Filters = append(ctx.Filters, bson.M{field.BSONField: bson.M{"$in": ids}})
			continue
		}

		// raise an error on a unsupported filter
		stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
	}

	// add sorting
	for _, sorter := range ctx.JSONAPIRequest.Sorting {
		// get direction
		descending := strings.HasPrefix(sorter, "-")

		// normalize sorter
		normalizedSorter := strings.TrimPrefix(sorter, "-")

		// find field
		field := c.Model.Meta().Attributes[normalizedSorter]
		if field == nil {
			stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid sorter "%s"`, normalizedSorter)))
		}

		// check whitelist
		if !Contains(c.Sorters, field.Name) {
			stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`unsupported sorter "%s"`, normalizedSorter)))
		}

		// add sorter
		if descending {
			ctx.Sorting = append(ctx.Sorting, "-"+field.BSONField)
		} else {
			ctx.Sorting = append(ctx.Sorting, field.BSONField)
		}
	}

	// honor list limit
	if c.ListLimit > 0 && (ctx.JSONAPIRequest.PageSize == 0 || ctx.JSONAPIRequest.PageSize > c.ListLimit) {
		// restrain page size
		ctx.JSONAPIRequest.PageSize = c.ListLimit

		// enforce pagination
		if ctx.JSONAPIRequest.PageNumber == 0 {
			ctx.JSONAPIRequest.PageNumber = 1
		}
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// prepare slice
	slicePtr := c.Model.Meta().MakeSlice()

	// prepare query
	query := ctx.Store.C(c.Model).Find(ctx.Query()).Sort(ctx.Sorting...)

	// add pagination
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		query = query.Limit(int(ctx.JSONAPIRequest.PageSize)).Skip(int((ctx.JSONAPIRequest.PageNumber - 1) * ctx.JSONAPIRequest.PageSize))
	}

	// query db
	ctx.Tracer.Push("mgo/Query.All")
	ctx.Tracer.Tag("query", ctx.Query())
	stack.AbortIf(query.All(slicePtr))
	ctx.Tracer.Pop()

	// set models
	ctx.Models = coal.InitSlice(slicePtr)

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) assignData(ctx *Context, res *jsonapi.Resource) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.assignData")

	// prepare whitelist
	var whitelist []string

	// covert field names to attributes and relationships
	for _, field := range ctx.WritableFields {
		// get field
		f := ctx.Model.Meta().Fields[field]
		if f == nil {
			stack.Abort(fmt.Errorf("unknown writable field %s", field))
		}

		// add attributes and relationships
		if f.JSONKey != "" {
			whitelist = append(whitelist, f.JSONKey)
		} else if f.RelName != "" {
			whitelist = append(whitelist, f.RelName)
		}
	}

	// whitelist attributes
	attributes := make(jsonapi.Map)
	for name, value := range res.Attributes {
		// get field
		field := c.Model.Meta().Attributes[name]
		if field == nil {
			stack.Abort(jsonapi.BadRequest("invalid attribute"))
		}

		// check whitelist
		if !Contains(whitelist, name) {
			// ignore violation if soft protection is enabled
			if c.SoftProtection {
				continue
			}

			stack.Abort(jsonapi.BadRequest("attribute is not writable"))
		}

		// set attribute
		attributes[name] = value
	}

	// map attributes to struct
	stack.AbortIf(attributes.Assign(ctx.Model))

	// iterate relationships
	for name, rel := range res.Relationships {
		// get relationship
		field := c.Model.Meta().Relationships[name]
		if field == nil {
			stack.Abort(jsonapi.BadRequest("invalid relationship"))
		}

		// check whitelist
		if !Contains(whitelist, name) || (!field.ToOne && !field.ToMany) {
			// ignore violation if soft protection is enabled
			if c.SoftProtection {
				continue
			}

			stack.Abort(jsonapi.BadRequest("relationship is not writable"))
		}

		// assign relationship
		c.assignRelationship(ctx, name, rel, field)
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) assignRelationship(ctx *Context, name string, rel *jsonapi.Document, field *coal.Field) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.assignRelationship")

	// handle to-one relationship
	if field.ToOne {
		// prepare zero value
		var id bson.ObjectId

		// set and check id if available
		if rel.Data != nil && rel.Data.One != nil {
			// check type
			if rel.Data.One.Type != field.RelType {
				stack.Abort(jsonapi.BadRequest("resource type mismatch"))
			}

			// check id
			if !bson.IsObjectIdHex(rel.Data.One.ID) {
				stack.Abort(jsonapi.BadRequest("invalid relationship id"))
			}

			// extract id
			id = bson.ObjectIdHex(rel.Data.One.ID)
		}

		// set non optional id
		if !field.Optional {
			ctx.Model.MustSet(field.Name, id)
			return
		}

		// set valid optional id
		if id.Valid() {
			ctx.Model.MustSet(field.Name, &id)
			return
		}

		// set zero optional id
		ctx.Model.MustSet(field.Name, coal.N())
	}

	// handle to-many relationship
	if field.ToMany {
		// prepare ids
		ids := make([]bson.ObjectId, len(rel.Data.Many))

		// convert all ids
		for i, r := range rel.Data.Many {
			// check type
			if r.Type != field.RelType {
				stack.Abort(jsonapi.BadRequest("resource type mismatch"))
			}

			// check id
			if !bson.IsObjectIdHex(r.ID) {
				stack.Abort(jsonapi.BadRequest("invalid relationship id"))
			}

			// set id
			ids[i] = bson.ObjectIdHex(r.ID)
		}

		// set ids
		ctx.Model.MustSet(field.Name, ids)
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) updateModel(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.updateModel")

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// update model
	ctx.Tracer.Push("mgo/Collection.UpdateId")
	stack.AbortIf(ctx.Store.C(ctx.Model).UpdateId(ctx.Model.ID(), ctx.Model))
	ctx.Tracer.Pop()

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) resourceForModel(ctx *Context, model coal.Model) *jsonapi.Resource {
	// begin trace
	ctx.Tracer.Push("fire/Controller.resourceForModel")

	// prepare whitelist
	whitelist := make([]string, 0, len(ctx.ReadableFields))

	// covert field names to attributes and relationships
	for _, field := range ctx.ReadableFields {
		// get field
		f := model.Meta().Fields[field]
		if f == nil {
			stack.Abort(fmt.Errorf("unknown readable field %s", field))
		}

		// add attributes and relationships
		if f.JSONKey != "" {
			whitelist = append(whitelist, f.JSONKey)
		} else if f.RelName != "" {
			whitelist = append(whitelist, f.RelName)
		}
	}

	// create map from model
	m, err := jsonapi.StructToMap(model, whitelist)
	stack.AbortIf(err)

	// prepare resource
	resource := &jsonapi.Resource{
		Type:          model.Meta().PluralName,
		ID:            model.ID().Hex(),
		Attributes:    m,
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := "/" + model.Meta().PluralName + "/" + model.ID().Hex()
	if ctx.JSONAPIRequest.Prefix != "" {
		base = "/" + ctx.JSONAPIRequest.Prefix + base
	}

	// go through all relationships
	for _, field := range model.Meta().Fields {
		// check if relationship
		if !field.ToOne && !field.ToMany && !field.HasOne && !field.HasMany {
			continue
		}

		// check if whitelisted
		if !Contains(whitelist, field.RelName) {
			continue
		}

		// prepare relationship links
		links := &jsonapi.DocumentLinks{
			Self:    base + "/relationships/" + field.RelName,
			Related: base + "/" + field.RelName,
		}

		// handle to-one relationship
		if field.ToOne {
			// prepare reference
			var reference *jsonapi.Resource

			if field.Optional {
				// get and check optional field
				oid := model.MustGet(field.Name).(*bson.ObjectId)

				// create reference if id is available
				if oid != nil {
					reference = &jsonapi.Resource{
						Type: field.RelType,
						ID:   oid.Hex(),
					}
				}
			} else {
				// directly create reference
				reference = &jsonapi.Resource{
					Type: field.RelType,
					ID:   model.MustGet(field.Name).(bson.ObjectId).Hex(),
				}
			}

			// set links and reference
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Links: links,
				Data: &jsonapi.HybridResource{
					One: reference,
				},
			}
		} else if field.ToMany {
			// get ids
			ids := model.MustGet(field.Name).([]bson.ObjectId)

			// prepare references
			references := make([]*jsonapi.Resource, len(ids))

			// set all references
			for i, id := range ids {
				references[i] = &jsonapi.Resource{
					Type: field.RelType,
					ID:   id.Hex(),
				}
			}

			// set links and reference
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Links: links,
				Data: &jsonapi.HybridResource{
					Many: references,
				},
			}
		} else if field.HasOne {
			// get related controller
			rc := ctx.Group.controllers[field.RelType]
			if rc == nil {
				stack.Abort(fmt.Errorf("missing related controller %s", field.RelType))
			}

			// find relationship
			rel := rc.Model.Meta().Relationships[field.RelInverse]
			if rel == nil {
				stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", field.RelInverse))
			}

			// prepare query
			query := bson.M{
				rel.BSONField: model.ID(),
			}

			// exclude soft deleted records
			if rc.SoftDeleteField != "" {
				query[coal.F(rc.Model, rc.SoftDeleteField)] = nil
			}

			// load all referenced ids
			var ids []bson.ObjectId
			ctx.Tracer.Push("mgo/Query.Distinct")
			ctx.Tracer.Tag("query", query)
			err := ctx.Store.C(rc.Model).Find(query).Distinct("_id", &ids)
			stack.AbortIf(err)
			ctx.Tracer.Pop()
			if len(ids) > 1 {
				stack.Abort(fmt.Errorf("has one relationship returned more than one result"))
			}

			// prepare reference
			var reference *jsonapi.Resource

			// set reference
			if len(ids) == 1 {
				reference = &jsonapi.Resource{
					Type: rc.Model.Meta().PluralName,
					ID:   ids[0].Hex(),
				}
			}

			// set links and reference
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Links: links,
				Data: &jsonapi.HybridResource{
					One: reference,
				},
			}
		} else if field.HasMany {
			// get related controller
			rc := ctx.Group.controllers[field.RelType]
			if rc == nil {
				stack.Abort(fmt.Errorf("missing related controller %s", field.RelType))
			}

			// find relationship
			rel := rc.Model.Meta().Relationships[field.RelInverse]
			if rel == nil {
				stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", field.RelInverse))
			}

			// prepare query
			query := bson.M{
				rel.BSONField: bson.M{
					"$in": []bson.ObjectId{model.ID()},
				},
			}

			// exclude soft deleted records
			if rc.SoftDeleteField != "" {
				query[coal.F(rc.Model, rc.SoftDeleteField)] = nil
			}

			// load all referenced ids
			var ids []bson.ObjectId
			ctx.Tracer.Push("mgo/Query.Distinct")
			ctx.Tracer.Tag("query", query)
			err := ctx.Store.C(rc.Model).Find(query).Distinct("_id", &ids)
			stack.AbortIf(err)
			ctx.Tracer.Pop()

			// prepare references
			references := make([]*jsonapi.Resource, len(ids))

			// set all references
			for i, id := range ids {
				references[i] = &jsonapi.Resource{
					Type: rc.Model.Meta().PluralName,
					ID:   id.Hex(),
				}
			}

			// set links and references
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Links: links,
				Data: &jsonapi.HybridResource{
					Many: references,
				},
			}
		}
	}

	// finish trace
	ctx.Tracer.Pop()

	return resource
}

func (c *Controller) resourcesForModels(ctx *Context, models []coal.Model) []*jsonapi.Resource {
	// begin trace
	ctx.Tracer.Push("fire/Controller.resourceForModels")

	// prepare resources
	resources := make([]*jsonapi.Resource, len(models))

	// create resources
	for i, model := range models {
		resources[i] = c.resourceForModel(ctx, model)
	}

	// finish trace
	ctx.Tracer.Pop()

	return resources
}

func (c *Controller) listLinks(self string, ctx *Context) *jsonapi.DocumentLinks {
	// begin trace
	ctx.Tracer.Push("fire/Controller.listLinks")

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: self,
	}

	// add pagination links
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		// get total amount of resources
		ctx.Tracer.Push("mgo/Query.Count")
		ctx.Tracer.Tag("query", ctx.Query())
		n, err := ctx.Store.C(c.Model).Find(ctx.Query()).Count()
		stack.AbortIf(err)
		ctx.Tracer.Pop()

		// calculate last page
		lastPage := uint64(math.Ceil(float64(n) / float64(ctx.JSONAPIRequest.PageSize)))

		// add basic pagination links
		links.Self = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, ctx.JSONAPIRequest.PageNumber, ctx.JSONAPIRequest.PageSize)
		links.First = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, 1, ctx.JSONAPIRequest.PageSize)
		links.Last = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, lastPage, ctx.JSONAPIRequest.PageSize)

		// add previous link if not on first page
		if ctx.JSONAPIRequest.PageNumber > 1 {
			links.Previous = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, ctx.JSONAPIRequest.PageNumber-1, ctx.JSONAPIRequest.PageSize)
		}

		// add next link if not on last page
		if ctx.JSONAPIRequest.PageNumber < lastPage {
			links.Next = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, ctx.JSONAPIRequest.PageNumber+1, ctx.JSONAPIRequest.PageSize)
		}
	}

	// finish trace
	ctx.Tracer.Pop()

	return links
}

func (c *Controller) runCallbacks(list []*Callback, ctx *Context, errorStatus int) {
	// return early if list is empty
	if len(list) == 0 {
		return
	}

	// begin trace
	ctx.Tracer.Push("fire/Controller.runCallbacks")

	// run callbacks and handle errors
	for _, cb := range list {
		// check if callback should be run
		if !cb.Matcher(ctx) {
			continue
		}

		// call callback
		err := cb.Handler(ctx)
		if IsSafe(err) {
			stack.Abort(&jsonapi.Error{
				Status: errorStatus,
				Detail: err.Error(),
			})
		} else if err != nil {
			stack.Abort(err)
		}
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) runAction(a *Action, ctx *Context, errorStatus int) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.runAction")

	// check if callback can be run
	if !a.Callback.Matcher(ctx) {
		stack.Abort(fmt.Errorf("not supported"))
	}

	// call callback
	err := a.Callback.Handler(ctx)
	if IsSafe(err) {
		stack.Abort(&jsonapi.Error{
			Status: errorStatus,
			Detail: err.Error(),
		})
	} else if err != nil {
		stack.Abort(err)
	}

	// finish trace
	ctx.Tracer.Pop()
}
