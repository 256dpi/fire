package fire

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// A Controller provides a JSON API based interface to a model.
//
// Database transactions are automatically used for list, find, create, update
// and delete operations. The created session can be accessed through the
// context to use it in callbacks.
//
// Note: A controller must not be modified after being added to a group.
type Controller struct {
	// The model that this controller should provide (e.g. &Foo{}).
	Model coal.Model

	// The store that is used to retrieve and persist the model.
	Store *coal.Store

	// Supported may be set to limit the supported operations of a controller.
	// By default all operations are supported. If the operation is not
	// supported the request will be aborted with an unsupported method error.
	Supported Matcher

	// Filters is a list of fields that are filterable. Only fields that are
	// exposed and indexed should be made filterable.
	Filters []string

	// Sorters is a list of fields that are sortable. Only fields that are
	// exposed and indexed should be made sortable.
	Sorters []string

	// Authorizers authorize the requested operation on the requested resource
	// and are run before any models are loaded from the store. Returned errors
	// will cause the abortion of the request with an unauthorized status by
	// default.
	//
	// The callbacks are expected to return an error if the requester should be
	// informed about being unauthorized to access the resource, or add filters
	// to the context to only return accessible resources. The later improves
	// privacy as a protected resource would appear as being not found.
	Authorizers []*Callback

	// Modifiers are run to modify the model during Create, Update and Delete
	// operations after the model is loaded and the changed attributes have been
	// assigned during an Update but before the model is validated. Returned
	// errors will cause the abortion of the request with a bad request status
	// by default.
	//
	// The callbacks are expected to modify the model to ensure default values,
	// aggregate fields or in general add data to the model.
	Modifiers []*Callback

	// Validators are run to validate Create, Update and Delete operations
	// after the model is loaded, changed, modified and passed basic validation.
	// Returned errors will cause the abortion of the request with a bad request
	// status by default.
	//
	// The callbacks are expected to validate the model being created, updated
	// or deleted and return errors if the presented attributes or relationships
	// are invalid or do not comply with the stated requirements. Necessary
	// authorization checks should be repeated and now also include the model's
	// attributes and relationships. The model should not be further modified to
	// ensure that the validators do not influence each other.
	Validators []*Callback

	// Decorators are run after the models or model have been loaded from the
	// database for List and Find operations or the model has been saved or
	// updated for Create and Update operations. Returned errors will cause the
	// abortion of the request with an InternalServerError status by default.
	Decorators []*Callback

	// Notifiers are run before the final response is written to the client
	// and provide a chance to modify the response and notify other systems
	// about the applied changes. Returned errors will cause the abortion of the
	// request with an InternalServerError status by default.
	Notifiers []*Callback

	// ListLimit can be set to a value higher than 1 to enforce paginated
	// responses and restrain the page size to be within one and the limit.
	//
	// Note: Fire uses the "page[number]" and "page[size]" query parameters for
	// pagination.
	ListLimit int64

	// DocumentLimit defines the maximum allowed size of an incoming document.
	// The serve.ByteSize helper can be used to set the value.
	//
	// Default: 8M.
	DocumentLimit int64

	// ReadTimeout and WriteTimeout specify the timeouts for read and write
	// operations.
	//
	// Default: 30s.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// CollectionActions and ResourceActions are custom actions that are run
	// on the collection (e.g. "posts/delete-cache") or resource (e.g.
	// "users/1/recover-password"). The request context is forwarded to
	// the specified callback after running the authorizers. No validators,
	// modifiers, decorators and notifiers are run for these requests.
	CollectionActions map[string]*Action
	ResourceActions   map[string]*Action

	// TolerateViolations will prevent errors if a non-writable field is
	// changed during a Create or Update operation.
	TolerateViolations bool

	// IdempotentCreate can be set to true to enable the idempotent create
	// mechanism. When creating resources, clients have to generate and submit a
	// unique "create token". The controller will then first check if a document
	// with the supplied token has already been created. The controller will
	// determine the token field from the provided model using the
	// "fire-idempotent-create" flag. It is recommended to add a unique index on
	// the token field and also enable the soft delete mechanism to prevent
	// duplicates if a short-lived document has already been deleted.
	IdempotentCreate bool

	// ConsistentUpdate can be set to true to enable the consistent update
	// mechanism. When updating a resource, the client has to first load the
	// most recent resource and retain the server generated "update token" and
	// send it with the update in the defined update token field. The controller
	// will then check if the stored document still has the same token. The
	// controller will determine the token field from the provided model using
	// the "fire-consistent-update" flag.
	ConsistentUpdate bool

	// SoftDelete can be set to true to enable the soft delete mechanism. If
	// enabled, the controller will flag documents as deleted instead of
	// immediately removing them. It will also exclude soft deleted documents
	// from queries. The controller will determine the timestamp field from the
	// provided model using the "fire-soft-delete" flag. It is advised to create
	// a TTL index to delete the documents automatically after some timeout.
	SoftDelete bool

	parser jsonapi.Parser
	meta   *coal.Meta
}

func (c *Controller) prepare() {
	// check operations
	if c.Supported == nil {
		c.Supported = All()
	}

	// prepare parser
	c.parser = jsonapi.Parser{
		CollectionActions: make(map[string][]string),
		ResourceActions:   make(map[string][]string),
	}

	// cache meta
	c.meta = coal.GetMeta(c.Model)

	// add collection actions
	for name, action := range c.CollectionActions {
		// check collision
		if name == "" || coal.IsHex(name) {
			panic(fmt.Sprintf(`fire: invalid collection action "%s"`, name))
		}

		// set default body limit
		if action.BodyLimit == 0 {
			action.BodyLimit = serve.MustByteSize("8M")
		}

		// set default timeout
		if action.Timeout == 0 {
			action.Timeout = 30 * time.Second
		}

		// add action to parser
		c.parser.CollectionActions[name] = action.Methods
	}

	// add resource actions
	for name, action := range c.ResourceActions {
		// check collision
		if name == "" || name == "relationships" || c.meta.Relationships[name] != nil {
			panic(fmt.Sprintf(`fire: invalid resource action "%s"`, name))
		}

		// set default body limit
		if action.BodyLimit == 0 {
			action.BodyLimit = serve.MustByteSize("8M")
		}

		// set default timeout
		if action.Timeout == 0 {
			action.Timeout = 30 * time.Second
		}

		// add action to parser
		c.parser.ResourceActions[name] = action.Methods
	}

	// set default document limit
	if c.DocumentLimit == 0 {
		c.DocumentLimit = serve.MustByteSize("8M")
	}

	// set default timeouts
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}

	// check soft delete field
	if c.SoftDelete {
		fieldName := coal.L(c.Model, "fire-soft-delete", true)
		if c.meta.Fields[fieldName].Type.String() != "*time.Time" {
			panic(fmt.Sprintf(`fire: soft delete field "%s" for model "%s" is not of type "*time.Time"`, fieldName, c.meta.Name))
		}
	}

	// check idempotent create field
	if c.IdempotentCreate {
		fieldName := coal.L(c.Model, "fire-idempotent-create", true)
		if c.meta.Fields[fieldName].Type.String() != "string" {
			panic(fmt.Sprintf(`fire: idempotent create field "%s" for model "%s" is not of type "string"`, fieldName, c.meta.Name))
		}
	}

	// check consistent update field
	if c.ConsistentUpdate {
		fieldName := coal.L(c.Model, "fire-consistent-update", true)
		if c.meta.Fields[fieldName].Type.String() != "string" {
			panic(fmt.Sprintf(`fire: consistent update field "%s" for model "%s" is not of type "string"`, fieldName, c.meta.Name))
		}
	}
}

func (c *Controller) handle(prefix string, ctx *Context, selector bson.M, write bool) {
	// trace
	ctx.Tracer.Push("fire/Controller.handle")
	defer ctx.Tracer.Pop()

	// prepare parser
	parser := c.parser
	parser.Prefix = prefix

	// parse incoming JSON-API request if not yet present
	if ctx.JSONAPIRequest == nil {
		req, err := parser.ParseRequest(ctx.HTTPRequest)
		xo.AbortIf(err)
		ctx.JSONAPIRequest = req
	}

	// parse document if not yet present and expected
	if ctx.Request == nil && ctx.JSONAPIRequest.Intent.DocumentExpected() {
		// limit request body size
		serve.LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, c.DocumentLimit)

		// parse document and respect document limit
		doc, err := jsonapi.ParseDocument(ctx.HTTPRequest.Body)
		xo.AbortIf(err)

		// set document
		ctx.Request = doc
	}

	// validate id if present
	if ctx.JSONAPIRequest.ResourceID != "" && !coal.IsHex(ctx.JSONAPIRequest.ResourceID) {
		xo.Abort(jsonapi.BadRequest("invalid resource id"))
	}

	// set operation
	switch ctx.JSONAPIRequest.Intent {
	case jsonapi.ListResources:
		ctx.Operation = List
	case jsonapi.FindResource:
		ctx.Operation = Find
	case jsonapi.CreateResource:
		ctx.Operation = Create
	case jsonapi.UpdateResource:
		ctx.Operation = Update
	case jsonapi.DeleteResource:
		ctx.Operation = Delete
	case jsonapi.GetRelatedResources, jsonapi.GetRelationship:
		ctx.Operation = Find
	case jsonapi.SetRelationship, jsonapi.AppendToRelationship, jsonapi.RemoveFromRelationship:
		ctx.Operation = Update
	case jsonapi.CollectionAction:
		ctx.Operation = CollectionAction
	case jsonapi.ResourceAction:
		ctx.Operation = ResourceAction
	}

	// check if supported
	if !c.Supported(ctx) {
		xo.Abort(jsonapi.ErrorFromStatus(
			http.StatusMethodNotAllowed,
			"unsupported operation",
		))
	}

	// check selector
	if selector == nil {
		selector = bson.M{}
	}

	// prepare context
	ctx.Selector = selector
	ctx.Filters = []bson.M{}
	ctx.ReadableFields = c.initialFields(false, ctx.JSONAPIRequest)
	ctx.WritableFields = c.initialFields(true, nil)
	ctx.RelationshipFilters = map[string][]bson.M{}

	// set store
	ctx.Store = c.Store

	// run operation with transaction if not an action
	if !ctx.Operation.Action() {
		xo.AbortIf(c.Store.T(ctx.Context, ctx.Operation.Read(), func(tc context.Context) error {
			ctx.With(tc, func() {
				c.runOperation(ctx)
			})
			return nil
		}))
	} else {
		c.runOperation(ctx)
	}

	// write response if available
	if write && ctx.Response != nil {
		xo.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, ctx.ResponseCode, ctx.Response))
	}
}

func (c *Controller) runOperation(ctx *Context) {
	// call specific handlers
	switch ctx.JSONAPIRequest.Intent {
	case jsonapi.ListResources:
		c.listResources(ctx)
	case jsonapi.FindResource:
		c.findResource(ctx)
	case jsonapi.CreateResource:
		c.createResource(ctx)
	case jsonapi.UpdateResource:
		c.updateResource(ctx)
	case jsonapi.DeleteResource:
		c.deleteResource(ctx)
	case jsonapi.GetRelatedResources:
		c.getRelatedResources(ctx)
	case jsonapi.GetRelationship:
		c.getRelationship(ctx)
	case jsonapi.SetRelationship:
		c.setRelationship(ctx)
	case jsonapi.AppendToRelationship:
		c.appendToRelationship(ctx)
	case jsonapi.RemoveFromRelationship:
		c.removeFromRelationship(ctx)
	case jsonapi.CollectionAction:
		c.handleCollectionAction(ctx)
	case jsonapi.ResourceAction:
		c.handleResourceAction(ctx)
	}
}

func (c *Controller) listResources(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.listResources")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.ReadTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// load models
	c.loadModels(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, ctx.Models)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			Many: c.resourcesForModels(ctx, ctx.Models, relationships),
		},
		Links: c.listLinks(ctx.JSONAPIRequest.Self(), ctx),
	}
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) findResource(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.findResource")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.ReadTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// load model
	c.loadModel(ctx)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			One: c.resourceForModel(ctx, ctx.Model, relationships),
		},
		Links: &jsonapi.DocumentLinks{
			Self: ctx.JSONAPIRequest.Self(),
		},
	}
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) createResource(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.createResource")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// basic input data check
	if ctx.Request.Data == nil || ctx.Request.Data.One == nil {
		xo.Abort(jsonapi.BadRequest("missing document"))
	}

	// check resource type
	if ctx.Request.Data.One.Type != ctx.JSONAPIRequest.ResourceType {
		xo.Abort(jsonapi.BadRequest("resource type mismatch"))
	}

	// check id
	if ctx.Request.Data.One.ID != "" {
		xo.Abort(jsonapi.BadRequest("unnecessary resource id"))
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// create model with id
	ctx.Model = c.meta.Make()
	ctx.Model.GetBase().DocID = coal.New()

	// assign attributes
	c.assignData(ctx, ctx.Request.Data.One)

	// run modifiers
	c.runCallbacks(c.Modifiers, ctx, http.StatusBadRequest)

	// validate model
	err := ctx.Model.Validate()
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// set initial update token if consistent update is enabled
	if c.ConsistentUpdate {
		consistentUpdateField := coal.L(ctx.Model, "fire-consistent-update", true)
		stick.MustSet(ctx.Model, consistentUpdateField, coal.New().Hex())
	}

	// check if idempotent create is enabled
	if c.IdempotentCreate {
		// get idempotent create field
		idempotentCreateField := coal.L(ctx.Model, "fire-idempotent-create", true)

		// get supplied idempotent create token
		idempotentCreateToken := stick.MustGet(ctx.Model, idempotentCreateField).(string)
		if idempotentCreateToken == "" {
			xo.Abort(jsonapi.BadRequest("missing idempotent create token"))
		}

		// insert model
		inserted, err := ctx.Store.M(c.Model).InsertIfMissing(ctx, bson.M{
			idempotentCreateField: idempotentCreateToken,
		}, ctx.Model, false)
		if coal.IsDuplicate(err) {
			xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
		}
		xo.AbortIf(err)

		// fail if not inserted
		if !inserted {
			xo.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "existing document with same idempotent create token"))
		}
	} else {
		// insert model
		err := ctx.Store.M(c.Model).Insert(ctx, ctx.Model)
		if coal.IsDuplicate(err) {
			xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
		}
		xo.AbortIf(err)
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			One: c.resourceForModel(ctx, ctx.Model, nil),
		},
		Links: &jsonapi.DocumentLinks{
			Self: ctx.JSONAPIRequest.Self() + "/" + ctx.Model.ID().Hex(),
		},
	}
	ctx.ResponseCode = http.StatusCreated

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) updateResource(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.updateResource")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// basic input data check
	if ctx.Request.Data == nil || ctx.Request.Data.One == nil {
		xo.Abort(jsonapi.BadRequest("missing document"))
	}

	// check resource type
	if ctx.Request.Data.One.Type != ctx.JSONAPIRequest.ResourceType {
		xo.Abort(jsonapi.BadRequest("resource type mismatch"))
	}

	// check id
	if ctx.Request.Data.One.ID != ctx.JSONAPIRequest.ResourceID {
		xo.Abort(jsonapi.BadRequest("resource id mismatch"))
	}

	// load model
	c.loadModel(ctx)

	// get stored idempotent create token
	var storedIdempotentCreateToken string
	if c.IdempotentCreate {
		idempotentCreateField := coal.L(ctx.Model, "fire-idempotent-create", true)
		storedIdempotentCreateToken = stick.MustGet(ctx.Model, idempotentCreateField).(string)
	}

	// get and reset stored consistent update token
	var storedConsistentUpdateToken string
	if c.ConsistentUpdate {
		consistentUpdateField := coal.L(ctx.Model, "fire-consistent-update", true)
		storedConsistentUpdateToken = stick.MustGet(ctx.Model, consistentUpdateField).(string)
		stick.MustSet(ctx.Model, consistentUpdateField, "")
	}

	// assign attributes
	c.assignData(ctx, ctx.Request.Data.One)

	// run modifiers
	c.runCallbacks(c.Modifiers, ctx, http.StatusBadRequest)

	// validate model
	err := ctx.Model.Validate()
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// check if idempotent create token has been changed
	if c.IdempotentCreate {
		idempotentCreateField := coal.L(ctx.Model, "fire-idempotent-create", true)
		idempotentCreateToken := stick.MustGet(ctx.Model, idempotentCreateField).(string)
		if storedIdempotentCreateToken != idempotentCreateToken {
			xo.Abort(jsonapi.BadRequest("idempotent create token cannot be changed"))
		}
	}

	// check if consistent update is enabled
	if c.ConsistentUpdate {
		// get consistency update field
		consistentUpdateField := coal.L(ctx.Model, "fire-consistent-update", true)

		// get consistent update token
		consistentUpdateToken := stick.MustGet(ctx.Model, consistentUpdateField).(string)
		if consistentUpdateToken != storedConsistentUpdateToken {
			xo.Abort(jsonapi.BadRequest("invalid consistent update token"))
		}

		// generate new update token
		stick.MustSet(ctx.Model, consistentUpdateField, coal.New().Hex())

		// update model
		found, err := ctx.Store.M(c.Model).ReplaceFirst(ctx, bson.M{
			"_id":                 ctx.Model.ID(),
			consistentUpdateField: consistentUpdateToken,
		}, ctx.Model, false)
		if coal.IsDuplicate(err) {
			xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
		}
		xo.AbortIf(err)

		// fail if not found
		if !found {
			xo.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "existing document with different consistent update token"))
		}
	} else {
		// replace model
		found, err := ctx.Store.M(c.Model).Replace(ctx, ctx.Model, false)
		if coal.IsDuplicate(err) {
			xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
		}
		xo.AbortIf(err)

		// check if missing
		if !found {
			xo.Abort(jsonapi.NotFound("resource not found"))
		}
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			One: c.resourceForModel(ctx, ctx.Model, relationships),
		},
		Links: &jsonapi.DocumentLinks{
			Self: ctx.JSONAPIRequest.Self(),
		},
	}
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) deleteResource(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.deleteResource")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// load model
	c.loadModel(ctx)

	// run modifiers
	c.runCallbacks(c.Modifiers, ctx, http.StatusBadRequest)

	// validate model
	err := ctx.Model.Validate()
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// check if soft delete has been enabled
	if c.SoftDelete {
		// get soft delete field
		softDeleteField := coal.L(c.Model, "fire-soft-delete", true)

		// soft delete model
		found, err := ctx.Store.M(c.Model).Update(ctx, nil, ctx.Model.ID(), bson.M{
			"$set": bson.M{
				softDeleteField: time.Now(),
			},
		}, false)
		xo.AbortIf(err)

		// check if missing
		if !found {
			xo.Abort(jsonapi.NotFound("resource not found"))
		}
	} else {
		// delete model
		found, err := ctx.Store.M(c.Model).Delete(ctx, nil, ctx.Model.ID())
		xo.AbortIf(err)

		// check if missing
		if !found {
			xo.Abort(jsonapi.NotFound("resource not found"))
		}
	}

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// set status
	ctx.ResponseWriter.WriteHeader(http.StatusNoContent)
}

func (c *Controller) getRelatedResources(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.getRelatedResources")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.ReadTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// find relationship
	rel := c.meta.Relationships[ctx.JSONAPIRequest.RelatedResource]
	if rel == nil {
		xo.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is readable
	if !stick.Contains(ctx.ReadableFields, rel.Name) {
		xo.Abort(jsonapi.BadRequest("relationship is not readable"))
	}

	// get related controller
	rc := ctx.Group.controllers[rel.RelType]
	if rc == nil {
		xo.Abort(xo.F("missing related controller for %s", rel.RelType))
	}

	// prepare sub context
	subCtx := &Context{
		Context:        ctx,
		Data:           stick.Map{},
		HTTPRequest:    ctx.HTTPRequest,
		ResponseWriter: nil,
		Controller:     rc,
		Group:          ctx.Group,
		Tracer:         ctx.Tracer,
	}

	// copy and prepare request
	req := *ctx.JSONAPIRequest
	req.Intent = jsonapi.ListResources
	req.ResourceType = rel.RelType
	req.ResourceID = ""
	req.RelatedResource = ""
	subCtx.JSONAPIRequest = &req

	// finish to-one relationship
	if rel.ToOne {
		// lookup id of related resource
		id := coal.New()
		if rel.Optional {
			oid := stick.MustGet(ctx.Model, rel.Name).(*coal.ID)
			if oid != nil {
				id = *oid
			}
		} else {
			id = stick.MustGet(ctx.Model, rel.Name).(coal.ID)
		}

		// prepare selector
		selector := bson.M{
			"_id": id,
		}

		// when run to request even if the relation is not present to capture
		// a proper response with potential meta information

		// handle virtual request
		rc.handle("", subCtx, selector, false)

		// check response
		if len(subCtx.Response.Data.Many) > 1 {
			xo.Abort(xo.F("to one relationship returned more than one result"))
		}

		// pull out resource
		if len(subCtx.Response.Data.Many) == 1 {
			subCtx.Response.Data.One = subCtx.Response.Data.Many[0]
		}

		// unset list
		subCtx.Response.Data.Many = nil
	}

	// finish to-many relationship
	if rel.ToMany {
		// get ids from loaded model
		ids := stick.MustGet(ctx.Model, rel.Name).([]coal.ID)

		// prepare selector
		selector := bson.M{
			"_id": bson.M{"$in": ids},
		}

		// handle virtual request
		rc.handle("", subCtx, selector, false)
	}

	// finish has-one relationship
	if rel.HasOne {
		// find inverse relationship
		inverse := rc.meta.Relationships[rel.RelInverse]
		if inverse == nil {
			xo.Abort(xo.F("no relationship matching the inverse name %s", inverse.RelInverse))
		}

		// prepare selector
		selector := bson.M{
			inverse.BSONKey: ctx.Model.ID(),
		}

		// handle virtual request
		rc.handle("", subCtx, selector, false)

		// check response
		if len(subCtx.Response.Data.Many) > 1 {
			xo.Abort(xo.F("has one relationship returned more than one result"))
		}

		// pull out resource
		if len(subCtx.Response.Data.Many) == 1 {
			subCtx.Response.Data.One = subCtx.Response.Data.Many[0]
		}

		// unset list
		subCtx.Response.Data.Many = nil
	}

	// finish has-many relationship
	if rel.HasMany {
		// find inverse relationship
		inverse := rc.meta.Relationships[rel.RelInverse]
		if inverse == nil {
			xo.Abort(xo.F("no relationship matching the inverse name %s", inverse.RelInverse))
		}

		// prepare selector
		selector := bson.M{
			inverse.BSONKey: bson.M{
				"$in": []coal.ID{ctx.Model.ID()},
			},
		}

		// handle virtual request
		rc.handle("", subCtx, selector, false)
	}

	// copy response
	ctx.Response = subCtx.Response
	ctx.ResponseCode = subCtx.ResponseCode

	// rewrite links
	from, to := subCtx.JSONAPIRequest.Self(), ctx.JSONAPIRequest.Self()
	ctx.Response.Links.Self = strings.Replace(ctx.Response.Links.Self, from, to, 1)
	ctx.Response.Links.Related = strings.Replace(ctx.Response.Links.Related, from, to, 1)
	ctx.Response.Links.First = strings.Replace(ctx.Response.Links.First, from, to, 1)
	ctx.Response.Links.Previous = strings.Replace(ctx.Response.Links.Previous, from, to, 1)
	ctx.Response.Links.Next = strings.Replace(ctx.Response.Links.Next, from, to, 1)
	ctx.Response.Links.Last = strings.Replace(ctx.Response.Links.Last, from, to, 1)
}

func (c *Controller) getRelationship(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.getRelationship")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.ReadTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// get relationship
	field := c.meta.Relationships[ctx.JSONAPIRequest.Relationship]
	if field == nil {
		xo.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is readable
	if !stick.Contains(ctx.ReadableFields, field.Name) {
		xo.Abort(jsonapi.BadRequest("relationship is not readable"))
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) setRelationship(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.setRelationship")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// abort if consistent update is enabled
	if c.ConsistentUpdate {
		xo.Abort(jsonapi.ErrorFromStatus(http.StatusMethodNotAllowed, "partial updates not allowed with consistent updates"))
	}

	// get relationship
	rel := c.meta.Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || (!rel.ToOne && !rel.ToMany) {
		xo.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !stick.Contains(ctx.WritableFields, rel.Name) {
		xo.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// assign relationship
	c.assignRelationship(ctx, ctx.Request, rel)

	// run modifiers
	c.runCallbacks(c.Modifiers, ctx, http.StatusBadRequest)

	// validate model
	err := ctx.Model.Validate()
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// replace model
	found, err := ctx.Store.M(c.Model).Replace(ctx, ctx.Model, false)
	if coal.IsDuplicate(err) {
		xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
	}
	xo.AbortIf(err)

	// check if missing
	if !found {
		xo.Abort(jsonapi.NotFound("resource not found"))
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) appendToRelationship(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.appendToRelationship")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// abort if consistent update is enabled
	if c.ConsistentUpdate {
		xo.Abort(jsonapi.ErrorFromStatus(http.StatusMethodNotAllowed, "partial updates not allowed with consistent updates"))
	}

	// get relationship
	rel := c.meta.Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || !rel.ToMany {
		xo.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !stick.Contains(ctx.WritableFields, rel.Name) {
		xo.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// process all references
	for _, ref := range ctx.Request.Data.Many {
		// check type
		if ref.Type != rel.RelType {
			xo.Abort(jsonapi.BadRequest("resource type mismatch"))
		}

		// get id
		refID, err := coal.FromHex(ref.ID)
		if err != nil {
			xo.Abort(jsonapi.BadRequest("invalid relationship id"))
		}

		// get current ids
		ids := stick.MustGet(ctx.Model, rel.Name).([]coal.ID)

		// check if id is already present
		if coal.Contains(ids, refID) {
			continue
		}

		// add id
		ids = append(ids, refID)
		stick.MustSet(ctx.Model, rel.Name, ids)
	}

	// run modifiers
	c.runCallbacks(c.Modifiers, ctx, http.StatusBadRequest)

	// validate model
	err := ctx.Model.Validate()
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// replace model
	found, err := ctx.Store.M(c.Model).Replace(ctx, ctx.Model, false)
	if coal.IsDuplicate(err) {
		xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
	}
	xo.AbortIf(err)

	// check if missing
	if !found {
		xo.Abort(jsonapi.NotFound("resource not found"))
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) removeFromRelationship(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.removeFromRelationship")
	defer ctx.Tracer.Pop()

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// abort if consistent update is enabled
	if c.ConsistentUpdate {
		xo.Abort(jsonapi.ErrorFromStatus(http.StatusMethodNotAllowed, "partial updates not allowed with consistent updates"))
	}

	// get relationship
	rel := c.meta.Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || !rel.ToMany {
		xo.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !stick.Contains(ctx.WritableFields, rel.Name) {
		xo.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// process all references
	for _, ref := range ctx.Request.Data.Many {
		// check type
		if ref.Type != rel.RelType {
			xo.Abort(jsonapi.BadRequest("resource type mismatch"))
		}

		// get id
		refID, err := coal.FromHex(ref.ID)
		if err != nil {
			xo.Abort(jsonapi.BadRequest("invalid relationship id"))
		}

		// prepare mark
		var pos = -1

		// get current ids
		ids := stick.MustGet(ctx.Model, rel.Name).([]coal.ID)

		// check if id is already present
		for i, id := range ids {
			if id == refID {
				pos = i
			}
		}

		// remove id if present
		if pos >= 0 {
			ids = append(ids[:pos], ids[pos+1:]...)
			stick.MustSet(ctx.Model, rel.Name, ids)
		}
	}

	// run modifiers
	c.runCallbacks(c.Modifiers, ctx, http.StatusBadRequest)

	// validate model
	err := ctx.Model.Validate()
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// replace model
	found, err := ctx.Store.M(c.Model).Replace(ctx, ctx.Model, false)
	if coal.IsDuplicate(err) {
		xo.Abort(jsonapi.ErrorFromStatus(http.StatusBadRequest, "document is not unique"))
	}
	xo.AbortIf(err)

	// check if missing
	if !found {
		xo.Abort(jsonapi.NotFound("resource not found"))
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]
	ctx.ResponseCode = http.StatusOK

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)
}

func (c *Controller) handleCollectionAction(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.handleCollectionAction")
	defer ctx.Tracer.Pop()

	// get action
	action, ok := c.CollectionActions[ctx.JSONAPIRequest.CollectionAction]
	if !ok {
		xo.Abort(xo.F("missing collection action callback"))
	}

	// limit request body size
	serve.LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, action.BodyLimit)

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, action.Timeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// run callback
	c.runAction(action, ctx, http.StatusBadRequest)
}

func (c *Controller) handleResourceAction(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.handleResourceAction")
	defer ctx.Tracer.Pop()

	// get action
	action, ok := c.ResourceActions[ctx.JSONAPIRequest.ResourceAction]
	if !ok {
		xo.Abort(xo.F("missing resource action callback"))
	}

	// limit request body size
	serve.LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, action.BodyLimit)

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, action.Timeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// load model
	c.loadModel(ctx)

	// run callback
	c.runAction(action, ctx, http.StatusBadRequest)
}

func (c *Controller) initialFields(write bool, r *jsonapi.Request) []string {
	// prepare list
	list := make([]string, 0, len(c.meta.Attributes)+len(c.meta.Relationships))

	// add attributes
	for _, f := range c.meta.Attributes {
		list = append(list, f.Name)
	}

	// add relationships
	for _, f := range c.meta.Relationships {
		if !write || f.ToOne || f.ToMany {
			list = append(list, f.Name)
		}
	}

	// check if a field whitelist has been provided
	if r != nil && len(r.Fields[c.meta.PluralName]) > 0 {
		// convert requested fields list
		var requested []string
		for _, field := range r.Fields[c.meta.PluralName] {
			// add attribute
			if f := c.meta.Attributes[field]; f != nil {
				requested = append(requested, f.Name)
				continue
			}

			// add relationship
			if f := c.meta.Relationships[field]; f != nil && (!write || f.ToOne || f.ToMany) {
				requested = append(requested, f.Name)
				continue
			}

			// raise error
			xo.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid sparse field "%s"`, field)))
		}

		// whitelist requested fields
		list = stick.Intersect(requested, list)
	}

	// sort list
	sort.Strings(list)

	return list
}

func (c *Controller) loadModel(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.loadModel")
	defer ctx.Tracer.Pop()

	// set selector query (id has been validated earlier)
	ctx.Selector["_id"] = coal.MustFromHex(ctx.JSONAPIRequest.ResourceID)

	// filter out deleted documents if configured
	if c.SoftDelete {
		// get soft delete field
		softDeleteField := coal.L(c.Model, "fire-soft-delete", true)

		// set filter
		ctx.Selector[softDeleteField] = nil
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// lock document if a write is expected
	lock := ctx.Operation.Write()

	// find model
	model := c.meta.Make()
	found, err := ctx.Store.M(c.Model).FindFirst(ctx, model, ctx.Query(), nil, 0, lock)
	xo.AbortIf(err)

	// check if missing
	if !found {
		xo.Abort(jsonapi.NotFound("resource not found"))
	}

	// set model
	ctx.Model = model

	// set original on update operations
	if ctx.Operation == Update {
		original := c.meta.Make()
		xo.AbortIf(stick.BSON.Transfer(model, original))
		ctx.Original = original
	}
}

func (c *Controller) loadModels(ctx *Context) {
	// trace
	ctx.Tracer.Push("fire/Controller.loadModels")
	defer ctx.Tracer.Pop()

	// filter out deleted documents if configured
	if c.SoftDelete {
		// get soft delete field
		softDeleteField := coal.L(c.Model, "fire-soft-delete", true)

		// set filter
		ctx.Selector[softDeleteField] = nil
	}

	// add filters
	for name, values := range ctx.JSONAPIRequest.Filters {
		// handle attributes filter
		if field := c.meta.Attributes[name]; field != nil {
			// check whitelist
			if !stick.Contains(c.Filters, field.Name) {
				xo.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
			}

			// handle boolean values
			if field.Kind == reflect.Bool && len(values) == 1 {
				ctx.Filters = append(ctx.Filters, bson.M{field.BSONKey: values[0] == "true"})
				continue
			}

			// handle string values
			ctx.Filters = append(ctx.Filters, bson.M{field.BSONKey: bson.M{"$in": values}})
			continue
		}

		// handle relationship filters
		if field := c.meta.Relationships[name]; field != nil {
			// check whitelist
			if !field.ToOne && !field.ToMany || !stick.Contains(c.Filters, field.Name) {
				xo.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
			}

			// convert to object ids
			var ids []coal.ID
			for _, str := range values {
				refID, err := coal.FromHex(str)
				if err != nil {
					xo.Abort(jsonapi.BadRequest("relationship filter value is not an object id"))
				}
				ids = append(ids, refID)
			}

			// set relationship filter
			ctx.Filters = append(ctx.Filters, bson.M{field.BSONKey: bson.M{"$in": ids}})
			continue
		}

		// raise an error on a unsupported filter
		xo.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
	}

	// add sorting
	for _, sorter := range ctx.JSONAPIRequest.Sorting {
		// get direction
		descending := strings.HasPrefix(sorter, "-")

		// normalize sorter
		normalizedSorter := strings.TrimPrefix(sorter, "-")

		// find field
		field := c.meta.Attributes[normalizedSorter]
		if field == nil {
			xo.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid sorter "%s"`, normalizedSorter)))
		}

		// check whitelist
		if !stick.Contains(c.Sorters, field.Name) {
			xo.Abort(jsonapi.BadRequest(fmt.Sprintf(`unsupported sorter "%s"`, normalizedSorter)))
		}

		// add sorter
		if descending {
			ctx.Sorting = append(ctx.Sorting, "-"+field.BSONKey)
		} else {
			ctx.Sorting = append(ctx.Sorting, field.BSONKey)
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

	// add pagination
	var skip, limit int64
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		limit = ctx.JSONAPIRequest.PageSize
		skip = (ctx.JSONAPIRequest.PageNumber - 1) * ctx.JSONAPIRequest.PageSize
	}

	// load documents
	models := c.meta.MakeSlice()
	xo.AbortIf(ctx.Store.M(c.Model).FindAll(ctx, models, ctx.Query(), ctx.Sorting, skip, limit, false))

	// set models
	ctx.Models = coal.Slice(models)
}

func (c *Controller) assignData(ctx *Context, res *jsonapi.Resource) {
	// trace
	ctx.Tracer.Push("fire/Controller.assignData")
	defer ctx.Tracer.Pop()

	// prepare whitelist
	var whitelist []string

	// covert field names to attributes and relationships
	for _, field := range ctx.WritableFields {
		// get field
		f := c.meta.Fields[field]
		if f == nil {
			xo.Abort(xo.F("unknown writable field %s", field))
		}

		// add attributes and relationships
		if f.JSONKey != "" {
			whitelist = append(whitelist, f.JSONKey)
		} else if f.RelName != "" {
			whitelist = append(whitelist, f.RelName)
		}
	}

	// prepare fields to verify read only access
	verifyReadOnly := make([]string, 0, len(res.Attributes)+len(res.Relationships))

	// whitelist attributes
	attributes := make(jsonapi.Map)
	for name, value := range res.Attributes {
		// get field
		field := c.meta.Attributes[name]
		if field == nil {
			pointer := fmt.Sprintf("/data/attributes/%s", name)
			xo.Abort(jsonapi.BadRequestPointer("invalid attribute", pointer))
		}

		// check whitelist
		if !stick.Contains(whitelist, name) {
			// ignore violation if tolerated or verify read only access
			if c.TolerateViolations {
				continue
			} else {
				verifyReadOnly = append(verifyReadOnly, field.Name)
			}
		}

		// set attribute
		attributes[name] = value
	}

	// map attributes to struct
	xo.AbortIf(attributes.Assign(ctx.Model))

	// iterate relationships
	for name, rel := range res.Relationships {
		// get relationship
		field := c.meta.Relationships[name]
		if field == nil {
			pointer := fmt.Sprintf("/data/relationships/%s", name)
			xo.Abort(jsonapi.BadRequestPointer("invalid relationship", pointer))
		}

		// check whitelist
		if !stick.Contains(whitelist, name) || (!field.ToOne && !field.ToMany) {
			// ignore violation if tolerated or verify read only access
			if c.TolerateViolations {
				continue
			} else {
				verifyReadOnly = append(verifyReadOnly, field.Name)
			}
		}

		// assign relationship
		c.assignRelationship(ctx, rel, field)
	}

	// verify read only fields
	for _, field := range verifyReadOnly {
		if ctx.Modified(field) {
			xo.Abort(jsonapi.BadRequestPointer("field is not writable", field))
		}
	}
}

func (c *Controller) assignRelationship(ctx *Context, rel *jsonapi.Document, field *coal.Field) {
	// trace
	ctx.Tracer.Push("fire/Controller.assignRelationship")
	defer ctx.Tracer.Pop()

	// handle to-one relationship
	if field.ToOne {
		// prepare zero value
		var id coal.ID

		// set and check id if available
		if rel.Data != nil && rel.Data.One != nil {
			// check type
			if rel.Data.One.Type != field.RelType {
				xo.Abort(jsonapi.BadRequest("resource type mismatch"))
			}

			// get id
			relID, err := coal.FromHex(rel.Data.One.ID)
			if err != nil {
				xo.Abort(jsonapi.BadRequest("invalid relationship id"))
			}

			// extract id
			id = relID
		}

		// set id properly
		if !field.Optional {
			stick.MustSet(ctx.Model, field.Name, id)
		} else {
			if !id.IsZero() {
				stick.MustSet(ctx.Model, field.Name, &id)
			} else {
				stick.MustSet(ctx.Model, field.Name, coal.N())
			}
		}
	}

	// handle to-many relationship
	if field.ToMany {
		// prepare ids
		var ids []coal.ID

		// check if data is available
		if rel.Data != nil {
			// prepare ids
			ids = make([]coal.ID, len(rel.Data.Many))

			// convert all ids
			for i, r := range rel.Data.Many {
				// check type
				if r.Type != field.RelType {
					xo.Abort(jsonapi.BadRequest("resource type mismatch"))
				}

				// get id
				relID, err := coal.FromHex(r.ID)
				if err != nil {
					xo.Abort(jsonapi.BadRequest("invalid relationship id"))
				}

				// set id
				ids[i] = relID
			}
		}

		// set ids
		stick.MustSet(ctx.Model, field.Name, ids)
	}
}

func (c *Controller) preloadRelationships(ctx *Context, models []coal.Model) map[string]map[coal.ID][]coal.ID {
	// trace
	ctx.Tracer.Push("fire/Controller.preloadRelationships")
	defer ctx.Tracer.Pop()

	// prepare relationships
	relationships := make(map[string]map[coal.ID][]coal.ID)

	// prepare whitelist
	whitelist := make([]string, 0, len(ctx.ReadableFields))

	// covert field names to relationships
	for _, field := range ctx.ReadableFields {
		// get field
		f := c.meta.Fields[field]
		if f == nil {
			xo.Abort(xo.F("unknown readable field %s", field))
		}

		// add relationships
		if f.RelName != "" {
			whitelist = append(whitelist, f.RelName)
		}
	}

	// go through all relationships
	for _, field := range c.meta.Relationships {
		// skip to one and to many relationships
		if field.ToOne || field.ToMany {
			continue
		}

		// check if whitelisted
		if !stick.Contains(whitelist, field.RelName) {
			continue
		}

		// get related controller
		rc := ctx.Group.controllers[field.RelType]
		if rc == nil {
			xo.Abort(xo.F("missing related controller %s", field.RelType))
		}

		// find relationship
		rel := rc.meta.Relationships[field.RelInverse]
		if rel == nil {
			xo.Abort(xo.F("no relationship matching the inverse name %s", field.RelInverse))
		}

		// collect model ids
		modelIDs := make([]coal.ID, 0, len(models))
		for _, model := range models {
			modelIDs = append(modelIDs, model.ID())
		}

		// prepare query
		query := bson.M{
			rel.BSONKey: bson.M{
				"$in": modelIDs,
			},
		}

		// exclude soft deleted documents
		if rc.SoftDelete {
			// get soft delete field
			softDeleteField := coal.L(rc.Model, "fire-soft-delete", true)

			// set filter
			query[softDeleteField] = nil
		}

		// prepare filters
		filters := []bson.M{query}

		// add relationship filters
		filters = append(filters, ctx.RelationshipFilters[field.Name]...)

		// project references
		references, err := ctx.Store.M(rc.Model).ProjectAll(ctx, bson.M{
			"$and": filters,
		}, rel.Name, nil, 0, 0, false)
		xo.AbortIf(err)

		// prepare entry
		entry := make(map[coal.ID][]coal.ID)

		// collect references
		for _, modelID := range modelIDs {
			// go through all related documents
			for id, value := range references {
				// handle to one references
				if rel.ToOne {
					// get reference id
					rid, _ := value.(coal.ID)
					if !rid.IsZero() && rid == modelID {
						// add reference
						entry[modelID] = append(entry[modelID], id)
					}
				}

				// handle to many references
				if rel.ToMany {
					// get reference ids
					rids, _ := value.(bson.A)
					for _, _rid := range rids {
						// get reference id
						rid, _ := _rid.(coal.ID)
						if !rid.IsZero() && rid == modelID {
							// add reference
							entry[modelID] = append(entry[modelID], id)
						}
					}
				}
			}
		}

		// sort references
		for _, refs := range entry {
			sort.Slice(refs, func(i, j int) bool {
				return bytes.Compare(refs[i][:], refs[j][:]) < 0
			})
		}

		// set references
		relationships[field.RelName] = entry
	}

	return relationships
}

func (c *Controller) resourceForModel(ctx *Context, model coal.Model, relationships map[string]map[coal.ID][]coal.ID) *jsonapi.Resource {
	// trace
	ctx.Tracer.Push("fire/Controller.resourceForModel")
	defer ctx.Tracer.Pop()

	// construct resource
	resource := c.constructResource(ctx, model, relationships)

	return resource
}

func (c *Controller) resourcesForModels(ctx *Context, models []coal.Model, relationships map[string]map[coal.ID][]coal.ID) []*jsonapi.Resource {
	// trace
	ctx.Tracer.Push("fire/Controller.resourceForModels")
	ctx.Tracer.Tag("count", len(models))
	defer ctx.Tracer.Pop()

	// prepare resources
	resources := make([]*jsonapi.Resource, len(models))

	// construct resources
	for i, model := range models {
		resources[i] = c.constructResource(ctx, model, relationships)
	}

	return resources
}

func (c *Controller) constructResource(ctx *Context, model coal.Model, relationships map[string]map[coal.ID][]coal.ID) *jsonapi.Resource {
	// do not trace this call

	// prepare whitelist
	whitelist := make([]string, 0, len(ctx.ReadableFields))

	// covert field names to attributes and relationships
	for _, field := range ctx.ReadableFields {
		// get field
		f := c.meta.Fields[field]
		if f == nil {
			xo.Abort(xo.F("unknown readable field %s", field))
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
	xo.AbortIf(err)

	// prepare resource
	resource := &jsonapi.Resource{
		Type:          c.meta.PluralName,
		ID:            model.ID().Hex(),
		Attributes:    m,
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := "/" + c.meta.PluralName + "/" + model.ID().Hex()
	if ctx.JSONAPIRequest.Prefix != "" {
		base = "/" + ctx.JSONAPIRequest.Prefix + base
	}

	// go through all relationships
	for _, field := range c.meta.Relationships {
		// check if whitelisted
		if !stick.Contains(whitelist, field.RelName) {
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
				oid := stick.MustGet(model, field.Name).(*coal.ID)

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
					ID:   stick.MustGet(model, field.Name).(coal.ID).Hex(),
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
			ids := stick.MustGet(model, field.Name).([]coal.ID)

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
			// skip if nil
			if relationships == nil {
				// set links and empty reference
				resource.Relationships[field.RelName] = &jsonapi.Document{
					Links: links,
					Data: &jsonapi.HybridResource{
						One: nil,
					},
				}

				continue
			}

			// get preloaded references
			refs, _ := relationships[field.RelName][model.ID()]

			// check length
			if len(refs) > 1 {
				xo.Abort(xo.F("has one relationship returned more than one result"))
			}

			// prepare reference
			var reference *jsonapi.Resource

			// set reference
			if len(refs) == 1 {
				reference = &jsonapi.Resource{
					Type: field.RelType,
					ID:   refs[0].Hex(),
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
			// skip if nil
			if relationships == nil {
				// set links and empty references
				resource.Relationships[field.RelName] = &jsonapi.Document{
					Links: links,
					Data: &jsonapi.HybridResource{
						Many: []*jsonapi.Resource{},
					},
				}

				continue
			}

			// get preloaded references
			refs, _ := relationships[field.RelName][model.ID()]

			// prepare references
			references := make([]*jsonapi.Resource, len(refs))

			// set all references
			for i, id := range refs {
				references[i] = &jsonapi.Resource{
					Type: field.RelType,
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

	return resource
}

func (c *Controller) listLinks(self string, ctx *Context) *jsonapi.DocumentLinks {
	// trace
	ctx.Tracer.Push("fire/Controller.listLinks")
	defer ctx.Tracer.Pop()

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: self,
	}

	// add pagination links
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		// count resources
		count, err := ctx.Store.M(c.Model).Count(ctx, ctx.Query(), 0, 0, false)
		xo.AbortIf(err)

		// calculate last page
		lastPage := int64(math.Ceil(float64(count) / float64(ctx.JSONAPIRequest.PageSize)))

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

	return links
}

func (c *Controller) runCallbacks(list []*Callback, ctx *Context, errorStatus int) {
	// return early if list is empty
	if len(list) == 0 {
		return
	}

	// trace
	ctx.Tracer.Push("fire/Controller.runCallbacks")
	defer ctx.Tracer.Pop()

	// run callbacks and handle errors
	for _, cb := range list {
		// check if callback should be run
		if !cb.Matcher(ctx) {
			continue
		}

		// call callback
		err := xo.W(cb.Handler(ctx))
		if xo.IsSafe(err) {
			xo.Abort(&jsonapi.Error{
				Status: errorStatus,
				Detail: err.Error(),
			})
		} else if err != nil {
			xo.Abort(err)
		}
	}
}

func (c *Controller) runAction(a *Action, ctx *Context, errorStatus int) {
	// trace
	ctx.Tracer.Push("fire/Controller.runAction")
	defer ctx.Tracer.Pop()

	// call action
	err := xo.W(a.Handler(ctx))
	if xo.IsSafe(err) {
		xo.Abort(&jsonapi.Error{
			Status: errorStatus,
			Detail: err.Error(),
		})
	} else if err != nil {
		xo.Abort(err)
	}
}
