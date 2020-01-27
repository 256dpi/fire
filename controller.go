package fire

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/lungo"
	"github.com/256dpi/serve"
	"github.com/256dpi/stack"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/256dpi/fire/coal"
)

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
	// and are run before any models are loaded from the store. Returned errors
	// will cause the abortion of the request with an unauthorized status by
	// default.
	//
	// The callbacks are expected to return an error if the requester should be
	// informed about being unauthorized to access the resource, or add filters
	// to the context to only return accessible resources. The later improves
	// privacy as a protected resource would appear as being not found.
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
	// abortion of the request with an InternalServerError status by default.
	Decorators []*Callback

	// Notifiers are run before the final response is written to the client
	// and provide a chance to modify the response and notify other systems
	// about the applied changes. Returned errors will cause the abortion of the
	// request with an InternalServerError status by default.
	Notifiers []*Callback

	// Disabled can be used to disable certain operations in general.
	Disabled []Operation

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
	// notifiers and decorators are run for the request.
	CollectionActions map[string]*Action
	ResourceActions   map[string]*Action

	// UseTransactions can be set to true to enable transactions for create,
	// update and delete operations. If enabled, a transaction will be created
	// and used for all database requests. The created session can be accessed
	// through the context to use it in callbacks.
	UseTransactions bool

	// TolerateViolations will not raise an error if a non-writable field is
	// set during a Create or Update operation. Frameworks like Ember.js just
	// serialize the complete state of a model and thus might send attributes
	// and relationships that are not writable.
	TolerateViolations bool

	// IdempotentCreate can be set to true to enable the idempotent create
	// mechanism. When creating resources, clients have to generate and submit a
	// unique "create token". The controller will then first check if a record
	// with the supplied token has already been created. The controller will
	// determine the token field from the provided model using the
	// "fire-idempotent-create" flag. It is recommended to add a unique index on
	// the token field and also enable the soft delete mechanism to prevent
	// duplicates if a short-lived record has already been deleted.
	IdempotentCreate bool

	// ConsistentUpdate can be set to true to enable the consistent update
	// mechanism. When updating a resource, the client has to first load the most
	// recent resource and retain the server generated "update token" and send
	// it with the update in the defined update token field. The controller will
	// then check if the stored record still has the same token. The controller
	// will determine the token field from the provided model using the
	// "fire-consistent-update" flag.
	ConsistentUpdate bool

	// SoftDelete can be set to true to enable the soft delete mechanism. If
	// enabled, the controller will flag documents as deleted instead of
	// immediately removing them. It will also exclude soft deleted documents
	// from queries. The controller will determine the timestamp field from the
	// provided model using the "fire-soft-delete" flag. It is advised to create
	// a TTL index to delete the documents automatically after some timeout.
	SoftDelete bool

	parser jsonapi.Parser
}

func (c *Controller) prepare() {
	// prepare parser
	c.parser = jsonapi.Parser{
		CollectionActions: make(map[string][]string),
		ResourceActions:   make(map[string][]string),
	}

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
		if name == "" || name == "relationships" || coal.GetMeta(c.Model).Relationships[name] != nil {
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
		if coal.GetMeta(c.Model).Fields[fieldName].Type.String() != "*time.Time" {
			panic(fmt.Sprintf(`fire: soft delete field "%s" for model "%s" is not of type "*time.Time"`, fieldName, coal.GetMeta(c.Model).Name))
		}
	}

	// check idempotent create field
	if c.IdempotentCreate {
		fieldName := coal.L(c.Model, "fire-idempotent-create", true)
		if coal.GetMeta(c.Model).Fields[fieldName].Type.String() != "string" {
			panic(fmt.Sprintf(`fire: idempotent create field "%s" for model "%s" is not of type "string"`, fieldName, coal.GetMeta(c.Model).Name))
		}
	}

	// check consistent update field
	if c.ConsistentUpdate {
		fieldName := coal.L(c.Model, "fire-consistent-update", true)
		if coal.GetMeta(c.Model).Fields[fieldName].Type.String() != "string" {
			panic(fmt.Sprintf(`fire: consistent update field "%s" for model "%s" is not of type "string"`, fieldName, coal.GetMeta(c.Model).Name))
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

	// parse document if expected
	var doc *jsonapi.Document
	if req.Intent.DocumentExpected() {
		// limit request body size
		serve.LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, c.DocumentLimit)

		// parse document and respect document limit
		doc, err = jsonapi.ParseDocument(ctx.HTTPRequest.Body)
		stack.AbortIf(err)
	}

	// set document
	ctx.Request = doc

	// validate id if present
	if req.ResourceID != "" && !coal.IsHex(req.ResourceID) {
		stack.Abort(jsonapi.BadRequest("invalid resource id"))
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

	// check if disabled
	for _, item := range c.Disabled {
		if item == ctx.Operation {
			stack.Abort(jsonapi.ErrorFromStatus(
				http.StatusMethodNotAllowed,
				"unsupported operation",
			))
		}
	}

	// prepare context
	ctx.Selector = bson.M{}
	ctx.Filters = []bson.M{}
	ctx.ReadableFields = c.initialFields(c.Model, ctx.JSONAPIRequest)
	ctx.WritableFields = c.initialFields(c.Model, nil)

	// set store
	ctx.Store = c.Store

	// run operation without transaction if not write or not configured
	if !ctx.Operation.Write() || !c.UseTransactions {
		c.runOperation(ctx, doc)
		ctx.Tracer.Pop()
		return
	}

	// run operation with transaction if enabled
	stack.AbortIf(c.Store.TX(ctx.Context, func(tc context.Context) error {
		// replace context
		ctx.Context = tc

		// run operation
		c.runOperation(ctx, doc)

		return nil
	}))

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) runOperation(ctx *Context, doc *jsonapi.Document) {
	// call specific handlers
	switch ctx.JSONAPIRequest.Intent {
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
}

func (c *Controller) listResources(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.listResources")

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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

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

	// create model with id
	ctx.Model = coal.GetMeta(c.Model).Make()
	ctx.Model.GetBase().DocID = coal.New()

	// assign attributes
	c.assignData(ctx, doc.Data.One)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// set initial update token if consistent update is enabled
	if c.ConsistentUpdate {
		consistentUpdateField := coal.L(ctx.Model, "fire-consistent-update", true)
		coal.MustSet(ctx.Model, consistentUpdateField, coal.New().Hex())
	}

	// check if idempotent create is enabled
	if c.IdempotentCreate {
		// get idempotent create field
		idempotentCreateField := coal.L(ctx.Model, "fire-idempotent-create", true)

		// get supplied idempotent create token
		idempotentCreateToken := coal.MustGet(ctx.Model, idempotentCreateField).(string)
		if idempotentCreateToken == "" {
			stack.Abort(jsonapi.BadRequest("missing idempotent create token"))
		}

		// insert model
		res, err := ctx.TC(ctx.Model).UpdateOne(ctx, bson.M{
			coal.F(ctx.Model, idempotentCreateField): idempotentCreateToken,
		}, bson.M{
			"$setOnInsert": ctx.Model,
		}, options.Update().SetUpsert(true))
		if lungo.IsUniquenessError(err) {
			stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
		}
		stack.AbortIf(err)

		// fail if already existing
		if res.MatchedCount != 0 {
			stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "existing document with same idempotent create token"))
		}
	} else {
		// insert model
		_, err := ctx.TC(ctx.Model).InsertOne(ctx, ctx.Model)
		if lungo.IsUniquenessError(err) {
			stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
		}
		stack.AbortIf(err)
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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

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

	// get stored idempotent create token
	var storedIdempotentCreateToken string
	if c.IdempotentCreate {
		idempotentCreateField := coal.L(ctx.Model, "fire-idempotent-create", true)
		storedIdempotentCreateToken = coal.MustGet(ctx.Model, idempotentCreateField).(string)
	}

	// get and reset stored consistent update token
	var storedConsistentUpdateToken string
	if c.ConsistentUpdate {
		consistentUpdateField := coal.L(ctx.Model, "fire-consistent-update", true)
		storedConsistentUpdateToken = coal.MustGet(ctx.Model, consistentUpdateField).(string)
		coal.MustSet(ctx.Model, consistentUpdateField, "")
	}

	// assign attributes
	c.assignData(ctx, doc.Data.One)

	// check if idempotent create token has been changed
	if c.IdempotentCreate {
		idempotentCreateField := coal.L(ctx.Model, "fire-idempotent-create", true)
		idempotentCreateToken := coal.MustGet(ctx.Model, idempotentCreateField).(string)
		if storedIdempotentCreateToken != idempotentCreateToken {
			stack.Abort(jsonapi.BadRequest("idempotent create token cannot be changed"))
		}
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// check if consistent update is enabled
	if c.ConsistentUpdate {
		// get consistency update field
		consistentUpdateField := coal.L(ctx.Model, "fire-consistent-update", true)

		// get consistent update token
		consistentUpdateToken := coal.MustGet(ctx.Model, consistentUpdateField).(string)
		if consistentUpdateToken != storedConsistentUpdateToken {
			stack.Abort(jsonapi.BadRequest("invalid consistent update token"))
		}

		// generate new update token
		coal.MustSet(ctx.Model, consistentUpdateField, coal.New().Hex())

		// update model
		res, err := ctx.TC(ctx.Model).UpdateOne(ctx, bson.M{
			"_id":                                    ctx.Model.ID(),
			coal.F(ctx.Model, consistentUpdateField): consistentUpdateToken,
		}, bson.M{
			"$set": ctx.Model,
		})
		if lungo.IsUniquenessError(err) {
			stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
		}
		stack.AbortIf(err)

		// fail if not updated
		if res.ModifiedCount != 1 {
			stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "existing document with different consistent update token"))
		}
	} else {
		// update model
		_, err := ctx.TC(ctx.Model).ReplaceOne(ctx, bson.M{
			"_id": ctx.Model.ID(),
		}, ctx.Model)
		if lungo.IsUniquenessError(err) {
			stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
		}
		stack.AbortIf(err)
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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// load model
	c.loadModel(ctx)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// check if soft delete has been enabled
	if c.SoftDelete {
		// get soft delete field
		softDeleteField := coal.L(c.Model, "fire-soft-delete", true)

		// soft delete model
		_, err := ctx.TC(c.Model).UpdateOne(ctx, bson.M{
			"_id": ctx.Model.ID(),
		}, bson.M{
			"$set": bson.M{
				coal.F(c.Model, softDeleteField): time.Now(),
			},
		})
		stack.AbortIf(err)
	} else {
		// remove model
		_, err := ctx.TC(c.Model).DeleteOne(ctx, bson.M{
			"_id": ctx.Model.ID(),
		})
		stack.AbortIf(err)
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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.ReadTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// find relationship
	rel := coal.GetMeta(c.Model).Relationships[ctx.JSONAPIRequest.RelatedResource]
	if rel == nil {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

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
		Data:           Map{},
		Selector:       bson.M{},
		Filters:        []bson.M{},
		ReadableFields: rc.initialFields(rc.Model, ctx.JSONAPIRequest),
		WritableFields: rc.initialFields(rc.Model, nil),
		Context:        ctx.Context,
		Request:        ctx.Request,
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
			oid := coal.MustGet(ctx.Model, rel.Name).(*coal.ID)
			if oid != nil {
				id = oid.Hex()
			}
		} else {
			id = coal.MustGet(ctx.Model, rel.Name).(coal.ID).Hex()
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

			// preload relationships
			relationships := c.preloadRelationships(newCtx, []coal.Model{newCtx.Model})

			// set model
			newCtx.Response.Data.One = rc.resourceForModel(newCtx, newCtx.Model, relationships)
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish to-many relationship
	if rel.ToMany {
		// get ids from loaded model
		ids := coal.MustGet(ctx.Model, rel.Name).([]coal.ID)

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query
		newCtx.Selector["_id"] = bson.M{"$in": ids}

		// load related models
		rc.loadModels(newCtx)

		// run decorators
		c.runCallbacks(c.Decorators, newCtx, http.StatusInternalServerError)

		// preload relationships
		relationships := rc.preloadRelationships(newCtx, newCtx.Models)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				Many: rc.resourcesForModels(newCtx, newCtx.Models, relationships),
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
		relRel := coal.GetMeta(rc.Model).Relationships[rel.RelInverse]
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
			// preload relationships
			relationships := c.preloadRelationships(newCtx, []coal.Model{newCtx.Models[0]})

			// set model
			newCtx.Response.Data.One = rc.resourceForModel(newCtx, newCtx.Models[0], relationships)
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish has-many relationship
	if rel.HasMany {
		// find relationship
		relRel := coal.GetMeta(rc.Model).Relationships[rel.RelInverse]
		if relRel == nil {
			stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", relRel.RelInverse))
		}

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query (supports to-one and to-many relationships)
		newCtx.Selector[relRel.BSONField] = bson.M{
			"$in": []coal.ID{ctx.Model.ID()},
		}

		// load related models
		rc.loadModels(newCtx)

		// run decorators
		c.runCallbacks(c.Decorators, newCtx, http.StatusInternalServerError)

		// preload relationships
		relationships := rc.preloadRelationships(newCtx, newCtx.Models)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				Many: rc.resourcesForModels(newCtx, newCtx.Models, relationships),
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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.ReadTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// get relationship
	field := coal.GetMeta(c.Model).Relationships[ctx.JSONAPIRequest.Relationship]
	if field == nil {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is readable
	if !Contains(ctx.ReadableFields, field.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not readable"))
	}

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// abort if consistent update is enabled
	if c.ConsistentUpdate {
		stack.Abort(jsonapi.ErrorFromStatus(http.StatusMethodNotAllowed, "partial updates not allowed with consistent updates"))
	}

	// get relationship
	rel := coal.GetMeta(c.Model).Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || (!rel.ToOne && !rel.ToMany) {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

	// load model
	c.loadModel(ctx)

	// check if relationship is writable
	if !Contains(ctx.WritableFields, rel.Name) {
		stack.Abort(jsonapi.BadRequest("relationship is not writable"))
	}

	// assign relationship
	c.assignRelationship(ctx, ctx.JSONAPIRequest.Relationship, doc, rel)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// update model
	_, err := ctx.TC(ctx.Model).ReplaceOne(ctx, bson.M{
		"_id": ctx.Model.ID(),
	}, ctx.Model)
	if lungo.IsUniquenessError(err) {
		stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
	}
	stack.AbortIf(err)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// abort if consistent update is enabled
	if c.ConsistentUpdate {
		stack.Abort(jsonapi.ErrorFromStatus(http.StatusMethodNotAllowed, "partial updates not allowed with consistent updates"))
	}

	// get relationship
	rel := coal.GetMeta(c.Model).Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || !rel.ToMany {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

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

		// get id
		refID, err := coal.FromHex(ref.ID)
		if err != nil {
			stack.Abort(jsonapi.BadRequest("invalid relationship id"))
		}

		// get current ids
		ids := coal.MustGet(ctx.Model, rel.Name).([]coal.ID)

		// check if id is already present
		if coal.Contains(ids, refID) {
			continue
		}

		// add id
		ids = append(ids, refID)
		coal.MustSet(ctx.Model, rel.Name, ids)
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// update model
	_, err := ctx.TC(ctx.Model).ReplaceOne(ctx, bson.M{
		"_id": ctx.Model.ID(),
	}, ctx.Model)
	if lungo.IsUniquenessError(err) {
		stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
	}
	stack.AbortIf(err)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

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

	// create context
	ct, cancel := context.WithTimeout(ctx.Context, c.WriteTimeout)
	defer cancel()

	// replace context
	ctx.Context = ct

	// abort if consistent update is enabled
	if c.ConsistentUpdate {
		stack.Abort(jsonapi.ErrorFromStatus(http.StatusMethodNotAllowed, "partial updates not allowed with consistent updates"))
	}

	// get relationship
	rel := coal.GetMeta(c.Model).Relationships[ctx.JSONAPIRequest.Relationship]
	if rel == nil || !rel.ToMany {
		stack.Abort(jsonapi.BadRequest("invalid relationship"))
	}

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

		// get id
		refID, err := coal.FromHex(ref.ID)
		if err != nil {
			stack.Abort(jsonapi.BadRequest("invalid relationship id"))
		}

		// prepare mark
		var pos = -1

		// get current ids
		ids := coal.MustGet(ctx.Model, rel.Name).([]coal.ID)

		// check if id is already present
		for i, id := range ids {
			if id == refID {
				pos = i
			}
		}

		// remove id if present
		if pos >= 0 {
			ids = append(ids[:pos], ids[pos+1:]...)
			coal.MustSet(ctx.Model, rel.Name, ids)
		}
	}

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// update model
	_, err := ctx.TC(ctx.Model).ReplaceOne(ctx, bson.M{
		"_id": ctx.Model.ID(),
	}, ctx.Model)
	if lungo.IsUniquenessError(err) {
		stack.Abort(jsonapi.ErrorFromStatus(http.StatusConflict, "document is not unique"))
	}
	stack.AbortIf(err)

	// run decorators
	c.runCallbacks(c.Decorators, ctx, http.StatusInternalServerError)

	// preload relationships
	relationships := c.preloadRelationships(ctx, []coal.Model{ctx.Model})

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model, relationships)

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

	// get action
	action, ok := c.CollectionActions[ctx.JSONAPIRequest.CollectionAction]
	if !ok {
		stack.Abort(fmt.Errorf("missing collection action callback"))
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

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) handleResourceAction(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.handleResourceAction")

	// get action
	action, ok := c.ResourceActions[ctx.JSONAPIRequest.ResourceAction]
	if !ok {
		stack.Abort(fmt.Errorf("missing resource action callback"))
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

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) initialFields(model coal.Model, r *jsonapi.Request) []string {
	// prepare list
	list := make([]string, 0, len(coal.GetMeta(model).Attributes)+len(coal.GetMeta(model).Relationships))

	// add attributes
	for _, f := range coal.GetMeta(model).Attributes {
		list = append(list, f.Name)
	}

	// add relationships
	for _, f := range coal.GetMeta(model).Relationships {
		list = append(list, f.Name)
	}

	// check if a field whitelist has been provided
	if r != nil && len(r.Fields[coal.GetMeta(model).PluralName]) > 0 {
		// convert requested fields list
		var requested []string
		for _, field := range r.Fields[coal.GetMeta(model).PluralName] {
			// add attribute
			if f := coal.GetMeta(model).Attributes[field]; f != nil {
				requested = append(requested, f.Name)
				continue
			}

			// add relationship
			if f := coal.GetMeta(model).Relationships[field]; f != nil {
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

	// set selector query (id has been validated earlier)
	ctx.Selector["_id"] = coal.MustFromHex(ctx.JSONAPIRequest.ResourceID)

	// filter out deleted documents if configured
	if c.SoftDelete {
		// get soft delete field
		softDeleteField := coal.L(c.Model, "fire-soft-delete", true)

		// set filter
		ctx.Selector[coal.F(c.Model, softDeleteField)] = nil
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// prepare object
	model := coal.GetMeta(c.Model).Make()

	// query db
	res := ctx.TC(c.Model).FindOne(ctx, ctx.Query())
	err := res.Decode(model)
	if err == mongo.ErrNoDocuments {
		stack.Abort(jsonapi.NotFound("resource not found"))
	}
	stack.AbortIf(err)

	// set model
	ctx.Model = model

	// set original on update operations
	if ctx.Operation == Update {
		original := coal.GetMeta(c.Model).Make()
		err = res.Decode(original)
		stack.AbortIf(err)
		ctx.Original = original
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) loadModels(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.loadModels")

	// filter out deleted documents if configured
	if c.SoftDelete {
		// get soft delete field
		softDeleteField := coal.L(c.Model, "fire-soft-delete", true)

		// set filter
		ctx.Selector[coal.F(c.Model, softDeleteField)] = nil
	}

	// add filters
	for name, values := range ctx.JSONAPIRequest.Filters {
		// handle attributes filter
		if field := coal.GetMeta(c.Model).Attributes[name]; field != nil {
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
		if field := coal.GetMeta(c.Model).Relationships[name]; field != nil {
			// check whitelist
			if !field.ToOne && !field.ToMany || !Contains(c.Filters, field.Name) {
				stack.Abort(jsonapi.BadRequest(fmt.Sprintf(`invalid filter "%s"`, name)))
			}

			// convert to object ids
			var ids []coal.ID
			for _, str := range values {
				refID, err := coal.FromHex(str)
				if err != nil {
					stack.Abort(jsonapi.BadRequest("relationship filter value is not an object id"))
				}
				ids = append(ids, refID)
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
		field := coal.GetMeta(c.Model).Attributes[normalizedSorter]
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
	slicePtr := coal.GetMeta(c.Model).MakeSlice()

	// prepare options
	opts := options.Find()

	// add sorting if present
	if len(ctx.Sorting) > 0 {
		opts = opts.SetSort(coal.Sort(ctx.Sorting...))
	}

	// add pagination
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		opts = opts.SetLimit(ctx.JSONAPIRequest.PageSize)
		opts = opts.SetSkip((ctx.JSONAPIRequest.PageNumber - 1) * ctx.JSONAPIRequest.PageSize)
	}

	// query db
	stack.AbortIf(ctx.TC(c.Model).FindAll(ctx, slicePtr, ctx.Query(), opts))

	// set models
	ctx.Models = coal.Slice(slicePtr)

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
		f := coal.GetMeta(ctx.Model).Fields[field]
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
		field := coal.GetMeta(c.Model).Attributes[name]
		if field == nil {
			pointer := fmt.Sprintf("/data/attributes/%s", name)
			stack.Abort(jsonapi.BadRequestPointer("invalid attribute", pointer))
		}

		// check whitelist
		if !Contains(whitelist, name) {
			// ignore violation if tolerated
			if c.TolerateViolations {
				continue
			}

			// raise error
			pointer := fmt.Sprintf("/data/attributes/%s", name)
			stack.Abort(jsonapi.BadRequestPointer("attribute is not writable", pointer))
		}

		// set attribute
		attributes[name] = value
	}

	// map attributes to struct
	stack.AbortIf(attributes.Assign(ctx.Model))

	// iterate relationships
	for name, rel := range res.Relationships {
		// get relationship
		field := coal.GetMeta(c.Model).Relationships[name]
		if field == nil {
			pointer := fmt.Sprintf("/data/relationships/%s", name)
			stack.Abort(jsonapi.BadRequestPointer("invalid relationship", pointer))
		}

		// check whitelist
		if !Contains(whitelist, name) || (!field.ToOne && !field.ToMany) {
			// ignore violation if tolerated
			if c.TolerateViolations {
				continue
			}

			// raise error
			pointer := fmt.Sprintf("/data/relationships/%s", name)
			stack.Abort(jsonapi.BadRequestPointer("relationship is not writable", pointer))
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
		var id coal.ID

		// set and check id if available
		if rel.Data != nil && rel.Data.One != nil {
			// check type
			if rel.Data.One.Type != field.RelType {
				stack.Abort(jsonapi.BadRequest("resource type mismatch"))
			}

			// get id
			relID, err := coal.FromHex(rel.Data.One.ID)
			if err != nil {
				stack.Abort(jsonapi.BadRequest("invalid relationship id"))
			}

			// extract id
			id = relID
		}

		// set id properly
		if !field.Optional {
			coal.MustSet(ctx.Model, field.Name, id)
		} else {
			if !id.IsZero() {
				coal.MustSet(ctx.Model, field.Name, &id)
			} else {
				coal.MustSet(ctx.Model, field.Name, coal.N())
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
					stack.Abort(jsonapi.BadRequest("resource type mismatch"))
				}

				// get id
				relID, err := coal.FromHex(r.ID)
				if err != nil {
					stack.Abort(jsonapi.BadRequest("invalid relationship id"))
				}

				// set id
				ids[i] = relID
			}
		}

		// set ids
		coal.MustSet(ctx.Model, field.Name, ids)
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) preloadRelationships(ctx *Context, models []coal.Model) map[string]map[coal.ID][]coal.ID {
	// begin trace
	ctx.Tracer.Push("fire/Controller.preloadRelationships")

	// prepare relationships
	relationships := make(map[string]map[coal.ID][]coal.ID)

	// prepare whitelist
	whitelist := make([]string, 0, len(ctx.ReadableFields))

	// covert field names to relationships
	for _, field := range ctx.ReadableFields {
		// get field
		f := coal.GetMeta(ctx.Controller.Model).Fields[field]
		if f == nil {
			stack.Abort(fmt.Errorf("unknown readable field %s", field))
		}

		// add relationships
		if f.RelName != "" {
			whitelist = append(whitelist, f.RelName)
		}
	}

	// go through all relationships
	for _, field := range coal.GetMeta(ctx.Controller.Model).Relationships {
		// skip to one and to many relationships
		if field.ToOne || field.ToMany {
			continue
		}

		// check if whitelisted
		if !Contains(whitelist, field.RelName) {
			continue
		}

		// get related controller
		rc := ctx.Group.controllers[field.RelType]
		if rc == nil {
			stack.Abort(fmt.Errorf("missing related controller %s", field.RelType))
		}

		// find relationship
		rel := coal.GetMeta(rc.Model).Relationships[field.RelInverse]
		if rel == nil {
			stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", field.RelInverse))
		}

		// collect model ids
		modelIDs := make([]coal.ID, 0, len(models))
		for _, model := range models {
			modelIDs = append(modelIDs, model.ID())
		}

		// prepare query
		query := bson.M{
			rel.BSONField: bson.M{
				"$in": modelIDs,
			},
		}

		// exclude soft deleted documents
		if rc.SoftDelete {
			// get soft delete field
			softDeleteField := coal.L(rc.Model, "fire-soft-delete", true)

			// set filter
			query[coal.F(rc.Model, softDeleteField)] = nil
		}

		// load all references
		var references []bson.M
		stack.AbortIf(ctx.TC(rc.Model).FindAll(ctx, &references, query, options.Find().SetProjection(bson.M{
			"_id":         1,
			rel.BSONField: 1,
		})))

		// prepare entry
		entry := make(map[coal.ID][]coal.ID)

		// collect references
		for _, modelID := range modelIDs {
			// go through all related documents
			for _, reference := range references {
				// handle to one references
				if rel.ToOne {
					// get reference id
					rid, _ := reference[rel.BSONField].(coal.ID)
					if !rid.IsZero() && rid == modelID {
						// add reference
						entry[modelID] = append(entry[modelID], reference["_id"].(coal.ID))
					}
				}

				// handle to many references
				if rel.ToMany {
					// get reference ids
					rids, _ := reference[rel.BSONField].(bson.A)
					for _, _rid := range rids {
						// get reference id
						rid, _ := _rid.(coal.ID)
						if !rid.IsZero() && rid == modelID {
							// add reference
							entry[modelID] = append(entry[modelID], reference["_id"].(coal.ID))
						}
					}
				}
			}
		}

		// set references
		relationships[field.RelName] = entry
	}

	// finish trace
	ctx.Tracer.Pop()

	return relationships
}

func (c *Controller) resourceForModel(ctx *Context, model coal.Model, relationships map[string]map[coal.ID][]coal.ID) *jsonapi.Resource {
	// begin trace
	ctx.Tracer.Push("fire/Controller.resourceForModel")

	// construct resource
	resource := c.constructResource(ctx, model, relationships)

	// finish trace
	ctx.Tracer.Pop()

	return resource
}

func (c *Controller) resourcesForModels(ctx *Context, models []coal.Model, relationships map[string]map[coal.ID][]coal.ID) []*jsonapi.Resource {
	// begin trace
	ctx.Tracer.Push("fire/Controller.resourceForModels")
	ctx.Tracer.Tag("count", len(models))

	// prepare resources
	resources := make([]*jsonapi.Resource, len(models))

	// construct resources
	for i, model := range models {
		resources[i] = c.constructResource(ctx, model, relationships)
	}

	// finish trace
	ctx.Tracer.Pop()

	return resources
}

func (c *Controller) constructResource(ctx *Context, model coal.Model, relationships map[string]map[coal.ID][]coal.ID) *jsonapi.Resource {
	// do not trace this call

	// prepare whitelist
	whitelist := make([]string, 0, len(ctx.ReadableFields))

	// covert field names to attributes and relationships
	for _, field := range ctx.ReadableFields {
		// get field
		f := coal.GetMeta(model).Fields[field]
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
		Type:          coal.GetMeta(model).PluralName,
		ID:            model.ID().Hex(),
		Attributes:    m,
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := "/" + coal.GetMeta(model).PluralName + "/" + model.ID().Hex()
	if ctx.JSONAPIRequest.Prefix != "" {
		base = "/" + ctx.JSONAPIRequest.Prefix + base
	}

	// go through all relationships
	for _, field := range coal.GetMeta(model).Relationships {
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
				oid := coal.MustGet(model, field.Name).(*coal.ID)

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
					ID:   coal.MustGet(model, field.Name).(coal.ID).Hex(),
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
			ids := coal.MustGet(model, field.Name).([]coal.ID)

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
				stack.Abort(fmt.Errorf("has one relationship returned more than one result"))
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
	// begin trace
	ctx.Tracer.Push("fire/Controller.listLinks")

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: self,
	}

	// add pagination links
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		// get total amount of resources
		n, err := ctx.TC(c.Model).CountDocuments(ctx, ctx.Query())
		stack.AbortIf(err)

		// calculate last page
		lastPage := int64(math.Ceil(float64(n) / float64(ctx.JSONAPIRequest.PageSize)))

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

	// call action
	err := a.Handler(ctx)
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
