package fire

import (
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strings"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
	"github.com/256dpi/stack"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
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
	// will cause the abortion of the request with an unauthorized status.
	//
	// The callbacks are expected to return an error if the requester should be
	// informed about him being unauthorized to access the resource or modify the
	// context's filter query in such a way that only accessible resources are
	// returned. The later improves privacy as a protected resource would just
	// appear as being not found.
	Authorizers []*Callback

	// Validators are run to validate Create, Update and Delete operations
	// after the models are loaded and the changed attributes have been assigned
	// during an Update. Returned errors will cause the abortion of the request
	// with a bad request status.
	//
	// The callbacks are expected to validate the model being created, updated or
	// deleted and return errors if the presented attributes or relationships
	// are invalid or do not comply with the stated requirements. The preceding
	// authorization checks should be repeated and now also include the model's
	// attributes and relationships.
	Validators []*Callback

	// Notifiers are run before the final response is written to the client
	// and provide a chance to modify the response and notify other systems
	// about the applied changes. Returned errors will cause the abortion of the
	// request with an Internal Server Error status.
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
		if name == "relationships" {
			panic(`fire: invalid resource action "relationships"`)
		}

		// check relations
		for _, field := range c.Model.Meta().Fields {
			if (field.ToOne || field.ToMany || field.HasOne || field.HasMany) && name == field.RelType {
				panic(fmt.Sprintf(`fire: invalid resource action "%s"`, name))
			}
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
			"Listing is disabled for this resource.",
		))
	}

	// parse document if expected
	var doc *jsonapi.Document
	if req.Intent.DocumentExpected() {
		// parse document and respect document limit
		doc, err = jsonapi.ParseDocument(http.MaxBytesReader(ctx.ResponseWriter, ctx.HTTPRequest.Body, int64(c.DocumentLimit)))
		stack.AbortIf(err)
	}

	// prepare context
	ctx.Selector = bson.M{}
	ctx.Filters = []bson.M{}
	ctx.Fields = c.initialFields(ctx.JSONAPIRequest, c.Model)

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
	models := c.loadModels(ctx)

	// compose response
	ctx.Response = &jsonapi.Document{
		Data: &jsonapi.HybridResource{
			Many: c.resourcesForModels(ctx, models),
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
	if doc.Data.One == nil {
		stack.Abort(jsonapi.BadRequest("resource object expected"))
	}

	// create new model
	ctx.Model = c.Model.Meta().Make()

	// assign attributes
	c.assignData(ctx, doc.Data.One)

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// insert model
	ctx.Tracer.Push("mgo/Collection.Insert")
	ctx.Tracer.Tag("model", ctx.Model)
	stack.AbortIf(ctx.Store.C(ctx.Model).Insert(ctx.Model))
	ctx.Tracer.Pop()

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
	if doc.Data.One == nil {
		stack.Abort(jsonapi.BadRequest("resource object expected"))
	}

	// load model
	c.loadModel(ctx)

	// assign attributes
	c.assignData(ctx, doc.Data.One)

	// save model
	c.updateModel(ctx)

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

	// remove model
	ctx.Tracer.Push("mgo/Collection.RemoveId")
	ctx.Tracer.Tag("model", ctx.Model)
	stack.AbortIf(ctx.Store.C(c.Model).RemoveId(ctx.Model.ID()))
	ctx.Tracer.Pop()

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

	// set operation
	ctx.Operation = Find

	// load model
	c.loadModel(ctx)

	// prepare resource type
	var relationField *coal.Field

	// find requested relationship
	for _, field := range ctx.Model.Meta().Fields {
		if field.RelName == ctx.JSONAPIRequest.RelatedResource {
			relationField = &field
			break
		}
	}

	// check resource type
	if relationField == nil {
		stack.Abort(jsonapi.BadRequest("relationship does not exist"))
	}

	// get related controller
	pluralName := relationField.RelType
	relatedController := ctx.Group.controllers[pluralName]

	// check related controller
	if relatedController == nil {
		stack.Abort(fmt.Errorf("missing related controller for %s", pluralName))
	}

	// copy context and request
	newCtx := &Context{
		Selector: bson.M{},
		Filters:  []bson.M{},
		Fields:   c.initialFields(ctx.JSONAPIRequest, relatedController.Model),
		Store:    ctx.Store,
		JSONAPIRequest: &jsonapi.Request{
			Prefix:       ctx.JSONAPIRequest.Prefix,
			ResourceType: pluralName,
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
		Controller:     relatedController,
		Group:          ctx.Group,
		Tracer:         ctx.Tracer,
	}

	// finish to-one relationship
	if relationField.ToOne {
		var id string

		// lookup id of related resource
		if relationField.Optional {
			// lookup optional id on loaded model
			oid := ctx.Model.MustGet(relationField.Name).(*bson.ObjectId)

			// TODO: Test present optional id.

			// check if present
			if oid != nil {
				id = oid.Hex()
			}
		} else {
			// lookup id on loaded model
			id = ctx.Model.MustGet(relationField.Name).(bson.ObjectId).Hex()
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
			relatedController.loadModel(newCtx)

			// set model
			newCtx.Response.Data.One = relatedController.resourceForModel(newCtx, newCtx.Model)
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish to-many relationship
	if relationField.ToMany {
		// get ids from loaded model
		ids := ctx.Model.MustGet(relationField.Name).([]bson.ObjectId)

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query
		newCtx.Selector["_id"] = bson.M{"$in": ids}

		// load related models
		models := relatedController.loadModels(newCtx)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				Many: relatedController.resourcesForModels(newCtx, models),
			},
			Links: relatedController.listLinks(ctx.JSONAPIRequest.Self(), newCtx),
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish has-one relationship
	if relationField.HasOne {
		// prepare filter
		var filterName string

		// find related relationship
		for _, field := range relatedController.Model.Meta().Fields {
			// find db field by comparing the relationship name wit the inverse
			// name found on the original relationship
			if field.RelName == relationField.RelInverse {
				filterName = field.BSONName
				break
			}
		}

		// check filter name
		if filterName == "" {
			stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", relationField.RelInverse))
		}

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query
		newCtx.Selector[filterName] = ctx.Model.ID()

		// load related models
		models := relatedController.loadModels(newCtx)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{},
			Links: &jsonapi.DocumentLinks{
				Self: ctx.JSONAPIRequest.Self(),
			},
		}

		// add if model is found
		if len(models) > 1 {
			stack.Abort(fmt.Errorf("has one relationship returned more than one result"))
		} else if len(models) == 1 {
			newCtx.Response.Data.One = relatedController.resourceForModel(newCtx, models[0])
		}

		// run notifiers
		c.runCallbacks(c.Notifiers, newCtx, http.StatusInternalServerError)

		// write result
		stack.AbortIf(jsonapi.WriteResponse(ctx.ResponseWriter, http.StatusOK, newCtx.Response))
	}

	// finish has-many relationship
	if relationField.HasMany {
		// prepare filter
		var filterName string

		// find related relationship
		for _, field := range relatedController.Model.Meta().Fields {
			// find db field by comparing the relationship name wit the inverse
			// name found on the original relationship
			if field.RelName == relationField.RelInverse {
				filterName = field.BSONName
				break
			}
		}

		// check filter name
		if filterName == "" {
			stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", relationField.RelInverse))
		}

		// tweak context
		newCtx.Operation = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set selector query (supports to-one and to-many relationships)
		newCtx.Selector[filterName] = bson.M{
			"$in": []bson.ObjectId{ctx.Model.ID()},
		}

		// load related models
		models := relatedController.loadModels(newCtx)

		// compose response
		newCtx.Response = &jsonapi.Document{
			Data: &jsonapi.HybridResource{
				Many: relatedController.resourcesForModels(newCtx, models),
			},
			Links: relatedController.listLinks(ctx.JSONAPIRequest.Self(), newCtx),
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

	// set operation
	ctx.Operation = Find

	// load model
	c.loadModel(ctx)

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

	// set operation
	ctx.Operation = Update

	// load model
	c.loadModel(ctx)

	// assign relationship
	c.assignRelationship(ctx, ctx.JSONAPIRequest.Relationship, doc)

	// save model
	c.updateModel(ctx)

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

	// set operation
	ctx.Operation = Update

	// load model
	c.loadModel(ctx)

	// append relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field matches relationship
		if !field.ToMany || field.RelName != ctx.JSONAPIRequest.Relationship {
			continue
		}

		// process all references
		for _, ref := range doc.Data.Many {
			// get id
			refID := bson.ObjectIdHex(ref.ID)

			// return error for an invalid id
			if !refID.Valid() {
				stack.Abort(jsonapi.BadRequest("invalid relationship id"))
			}

			// prepare mark
			var present bool

			// get current ids
			ids := ctx.Model.MustGet(field.Name).([]bson.ObjectId)

			// TODO: Test already existing reference.

			// check if id is already present
			for _, id := range ids {
				if id == refID {
					present = true
				}
			}

			// add id if not present
			if !present {
				ids = append(ids, refID)
				ctx.Model.MustSet(field.Name, ids)
			}
		}
	}

	// save model
	c.updateModel(ctx)

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

	// set operation
	ctx.Operation = Update

	// load model
	c.loadModel(ctx)

	// remove relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field matches relationship
		if !field.ToMany || field.RelName != ctx.JSONAPIRequest.Relationship {
			continue
		}

		// process all references
		for _, ref := range doc.Data.Many {
			// get id
			refID := bson.ObjectIdHex(ref.ID)

			// return error for an invalid id
			if !refID.Valid() {
				stack.Abort(jsonapi.BadRequest("invalid relationship id"))
			}

			// prepare mark
			var pos = -1

			// get current ids
			ids := ctx.Model.MustGet(field.Name).([]bson.ObjectId)

			// check if id is already present
			for i, id := range ids {
				if id == refID {
					pos = i
				}
			}

			// remove id if present
			if pos >= 0 {
				ids = append(ids[:pos], ids[pos+1:]...)
				ctx.Model.MustSet(field.Name, ids)
			}
		}
	}

	// save model
	c.updateModel(ctx)

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
		panic("fire: missing collection action callback")
	}

	// limit request body size
	ctx.HTTPRequest.Body = http.MaxBytesReader(ctx.ResponseWriter, ctx.HTTPRequest.Body, int64(action.BodyLimit))

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
		panic("fire: missing resource action callback")
	}

	// limit request body size
	ctx.HTTPRequest.Body = http.MaxBytesReader(ctx.ResponseWriter, ctx.HTTPRequest.Body, int64(action.BodyLimit))

	// load model
	c.loadModel(ctx)

	// run callback
	c.runAction(action, ctx, http.StatusBadRequest)

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) initialFields(r *jsonapi.Request, model coal.Model) []string {
	// prepare list
	var list []string

	// add all attributes and relationships
	for _, f := range model.Meta().Fields {
		if f.JSONName != "" || f.RelName != "" {
			list = append(list, f.Name)
		}
	}

	// check if a field whitelist has been provided
	if len(r.Fields[model.Meta().PluralName]) > 0 {
		// convert requested fields list
		var requested []string
		for _, field := range r.Fields[model.Meta().PluralName] {
			for _, f := range model.Meta().Fields {
				if f.JSONName == field || f.RelName == field {
					requested = append(requested, f.Name)
				}
			}
		}

		// whitelist requested fields
		list = Intersect(requested, list)
	}

	return list
}

func (c *Controller) loadModel(ctx *Context) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.loadModel")

	// validate id
	if !bson.IsObjectIdHex(ctx.JSONAPIRequest.ResourceID) {
		stack.Abort(jsonapi.BadRequest("invalid resource id"))
	}

	// set selector query
	ctx.Selector["_id"] = bson.ObjectIdHex(ctx.JSONAPIRequest.ResourceID)

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

func (c *Controller) loadModels(ctx *Context) []coal.Model {
	// begin trace
	ctx.Tracer.Push("fire/Controller.loadModels")

	// add filters
	for name, values := range ctx.JSONAPIRequest.Filters {
		// initialize flag
		handled := false

		for _, field := range c.Model.Meta().Fields {
			// handle attribute filter
			if field.JSONName == name && Contains(c.Filters, field.Name) {
				handled = true

				// handle boolean values
				if field.Kind == reflect.Bool && len(values) == 1 {
					ctx.Filters = append(ctx.Filters, bson.M{field.BSONName: values[0] == "true"})

					break
				}

				// handle string values
				ctx.Filters = append(ctx.Filters, bson.M{field.BSONName: bson.M{"$in": values}})

				break
			}

			// handle relationship filter
			if field.RelName == name && (field.ToOne || field.ToMany) && Contains(c.Filters, field.Name) {
				handled = true

				// convert to object id list
				ids, err := toObjectIDList(values)
				if err != nil {
					stack.Abort(jsonapi.BadRequest("relationship filter values are not object ids"))
				}

				// set relationship filter
				ctx.Filters = append(ctx.Filters, bson.M{field.BSONName: bson.M{"$in": ids}})

				break
			}
		}

		// raise an error on a unsupported filter
		if !handled {
			stack.Abort(jsonapi.BadRequest(fmt.Sprintf("filter %s is not supported", name)))
		}
	}

	// add sorting
	for _, sorter := range ctx.JSONAPIRequest.Sorting {
		// initialize flag
		handled := false

		// normalize sorter
		normalizedSorter := strings.TrimPrefix(sorter, "-")

		for _, field := range c.Model.Meta().Fields {
			// handle attribute sorter
			if (field.JSONName == normalizedSorter) && Contains(c.Sorters, field.Name) {
				handled = true

				// add sorter
				ctx.Sorting = append(ctx.Sorting, sorter)

				break
			}
		}

		// raise an error on a unsupported filter
		if !handled {
			stack.Abort(jsonapi.BadRequest(fmt.Sprintf("sorter %s is not supported", normalizedSorter)))
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

	// get models
	models := coal.InitSlice(slicePtr)

	// finish trace
	ctx.Tracer.Pop()

	return models
}

func (c *Controller) assignData(ctx *Context, res *jsonapi.Resource) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.assignData")

	// TODO: Only allow changes to whitelisted attributes?

	// map attributes to struct
	stack.AbortIf(res.Attributes.Assign(ctx.Model))

	// iterate relationships
	for name, rel := range res.Relationships {
		c.assignRelationship(ctx, name, rel)
	}

	// finish trace
	ctx.Tracer.Pop()
}

func (c *Controller) assignRelationship(ctx *Context, name string, rel *jsonapi.Document) {
	// begin trace
	ctx.Tracer.Push("fire/Controller.assignRelationship")

	// TODO: Only allow changes to whitelisted relationships?

	// assign relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field matches relationship
		if field.RelName != name || (!field.ToOne && !field.ToMany) {
			continue
		}

		// handle to-one relationship
		if field.ToOne {
			// prepare zero value
			var id bson.ObjectId

			// set and check id if available
			if rel.Data != nil && rel.Data.One != nil {
				id = bson.ObjectIdHex(rel.Data.One.ID)

				// return error for an invalid id
				if !id.Valid() {
					stack.Abort(jsonapi.BadRequest("invalid relationship id"))
				}
			}

			// handle non optional field first
			if !field.Optional {
				ctx.Model.MustSet(field.Name, id)
				continue
			}

			// assign for a zero value optional field
			if id != "" {
				ctx.Model.MustSet(field.Name, &id)
			} else {
				var nilID *bson.ObjectId
				ctx.Model.MustSet(field.Name, nilID)
			}
		}

		// handle to-many relationship
		if field.ToMany {
			// prepare slice of ids
			ids := make([]bson.ObjectId, len(rel.Data.Many))

			// range over all resources
			for i, r := range rel.Data.Many {
				// set id
				ids[i] = bson.ObjectIdHex(r.ID)

				// return error for an invalid id
				if !ids[i].Valid() {
					stack.Abort(jsonapi.BadRequest("invalid relationship id"))
				}
			}

			// set ids
			ctx.Model.MustSet(field.Name, ids)
		}
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

	// create whitelist
	var whitelist []string
	for _, field := range ctx.Fields {
		f := model.Meta().MustFindField(field)
		if f.JSONName != "" {
			whitelist = append(whitelist, f.JSONName)
		} else if f.RelName != "" {
			whitelist = append(whitelist, f.RelName)
		}
	}

	// TODO: Only expose whitelisted attributes.

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

	// TODO: Only expose whitelisted relationships.

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

			// assign relationship
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Data: &jsonapi.HybridResource{
					One: reference,
				},
				Links: links,
			}
		} else if field.ToMany {
			// get ids
			ids := model.MustGet(field.Name).([]bson.ObjectId)

			// prepare slice of references
			references := make([]*jsonapi.Resource, len(ids))

			// set all references
			for i, id := range ids {
				references[i] = &jsonapi.Resource{
					Type: field.RelType,
					ID:   id.Hex(),
				}
			}

			// assign relationship
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Data: &jsonapi.HybridResource{
					Many: references,
				},
				Links: links,
			}
		} else if field.HasOne {
			// get related controller
			relatedController := ctx.Group.controllers[field.RelType]

			// check existence
			if relatedController == nil {
				panic("fire: missing related controller " + field.RelType)
			}

			// prepare filter
			var filterName string

			// find related relationship
			for _, relatedField := range relatedController.Model.Meta().Fields {
				// find db field by comparing the relationship name wit the inverse
				// name found on the original relationship
				if relatedField.RelName == field.RelInverse {
					filterName = relatedField.BSONName
					break
				}
			}

			// check filter name
			if filterName == "" {
				stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", field.RelInverse))
			}

			// TODO: We should run the related controllers authenticator.
			// => Update comment on HasOne type.

			// prepare query
			query := bson.M{filterName: model.ID()}

			// load all referenced ids
			var ids []bson.ObjectId
			ctx.Tracer.Push("mgo/Query.Distinct")
			ctx.Tracer.Tag("query", query)
			err := ctx.Store.C(relatedController.Model).Find(query).Distinct("_id", &ids)
			stack.AbortIf(err)
			ctx.Tracer.Pop()

			// prepare references
			var reference *jsonapi.Resource

			// set all references
			if len(ids) > 1 {
				stack.Abort(fmt.Errorf("has one relationship returned more than one result"))
			} else if len(ids) == 1 {
				reference = &jsonapi.Resource{
					Type: relatedController.Model.Meta().PluralName,
					ID:   ids[0].Hex(),
				}
			}

			// only set links
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Links: links,
				Data: &jsonapi.HybridResource{
					One: reference,
				},
			}
		} else if field.HasMany {
			// get related controller
			relatedController := ctx.Group.controllers[field.RelType]

			// check existence
			if relatedController == nil {
				panic("fire: missing related controller " + field.RelType)
			}

			// prepare filter
			var filterName string

			// find related relationship
			for _, relatedField := range relatedController.Model.Meta().Fields {
				// find db field by comparing the relationship name wit the inverse
				// name found on the original relationship
				if relatedField.RelName == field.RelInverse {
					filterName = relatedField.BSONName
					break
				}
			}

			// check filter name
			if filterName == "" {
				stack.Abort(fmt.Errorf("no relationship matching the inverse name %s", field.RelInverse))
			}

			// TODO: We should run the related controllers authenticator.
			// => Update comment on HasMany type.

			// prepare query
			query := bson.M{
				filterName: bson.M{
					"$in": []bson.ObjectId{model.ID()},
				},
			}

			// load all referenced ids
			var ids []bson.ObjectId
			ctx.Tracer.Push("mgo/Query.Distinct")
			ctx.Tracer.Tag("query", query)
			err := ctx.Store.C(relatedController.Model).Find(query).Distinct("_id", &ids)
			stack.AbortIf(err)
			ctx.Tracer.Pop()

			// prepare references
			references := make([]*jsonapi.Resource, len(ids))

			// set all references
			for i, id := range ids {
				references[i] = &jsonapi.Resource{
					Type: relatedController.Model.Meta().PluralName,
					ID:   id.Hex(),
				}
			}

			// only set links
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
	resources := make([]*jsonapi.Resource, 0, len(models))

	// create resources
	for _, model := range models {
		resources = append(resources, c.resourceForModel(ctx, model))
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
		panic("fire: not supported")
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
