package fire

import (
	"fmt"
	"math"
	"net/http"
	"reflect"

	"github.com/256dpi/stack"
	"github.com/gonfire/jsonapi"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// A Controller provides a JSON API based interface to a model.
//
// Note: Controllers must not be modified after adding to an application.
type Controller struct {
	// The model that this controller should provide (e.g. &Foo{}).
	Model Model

	// Filters defines the attributes that are filterable.
	Filters []string

	// Sorters defines the attributes that are sortable.
	Sorters []string

	// The store that is used to retrieve and persist the model.
	Store *Store

	// The Authorizers authorize the requested action on the requested resource
	// and are run before any models are loaded from the DB. Returned errors
	// will cause the abortion of the request with an Unauthorized status.
	//
	// The callbacks are expected to return an error if the requester should be
	// informed about him being unauthorized to access the resource or modify the
	// Context's Query attribute in such a way that only accessible resources
	// are returned. The later improves privacy as a protected resource would
	// just appear as being not found.
	Authorizers []Callback

	// The Validators are run to validate Create, Update and Delete actions after
	// the models are loaded from the DB and the changed attributes have been
	// assigned during an Update. Returned errors will cause the abortion of the
	// request with a Bad Request status.
	//
	// The callbacks are expected to validate the Model being created, updated or
	// deleted and return errors if the presented attributes or relationships
	// are invalid or do not comply with the stated requirements. The preceding
	// authorization checks should be repeated and now also include the model's
	// attributes and relationships.
	Validators []Callback

	// The Notifiers are run before the final response is written to the client
	// and provide a chance to modify the response and notify other systems
	// about the applied changes. Returned errors will cause the abortion of the
	// request with an Internal Server Error status.
	Notifiers []Callback

	// The NoList property can be set to true if the resource is only listed
	// through relationships from other resources. This is useful for
	// resources like comments that should never be listed alone.
	NoList bool

	// The ListLimit can be set to a value higher than 1 to enforce paginated
	// responses and restrain the page size to be within one and the limit.
	ListLimit int
}

func (c *Controller) generalHandler(group *Group, prefix string, w http.ResponseWriter, r *http.Request) {
	// parse incoming JSON API request
	req, err := jsonapi.ParseRequest(r, prefix)
	stack.AbortIf(err)

	// handle no list setting
	if req.Intent == jsonapi.ListResources && c.NoList {
		stack.Abort(jsonapi.ErrorFromStatus(
			http.StatusMethodNotAllowed,
			"Listing is disabled for this resource.",
		))
	}

	// parse body if available
	var doc *jsonapi.Document
	if req.Intent.DocumentExpected() {
		doc, err = jsonapi.ParseDocument(r.Body)
		stack.AbortIf(err)
	}

	// copy store
	store := c.Store.Copy()

	// ensure store will be closed
	defer store.Close()

	// build context
	ctx := &Context{
		JSONAPIRequest: req,
		HTTPRequest:    r,
		prefix:         prefix,
		Controller:     c,
		Group:          group,
		Store:          store,
	}

	// call specific handlers based on the request intent
	switch req.Intent {
	case jsonapi.ListResources:
		ctx.Action = List
		c.listResources(w, ctx)
	case jsonapi.FindResource:
		ctx.Action = Find
		c.findResource(w, ctx)
	case jsonapi.CreateResource:
		ctx.Action = Create
		c.createResource(w, ctx, doc)
	case jsonapi.UpdateResource:
		ctx.Action = Update
		c.updateResource(w, ctx, doc)
	case jsonapi.DeleteResource:
		ctx.Action = Delete
		c.deleteResource(w, ctx)
	case jsonapi.GetRelatedResources:
		ctx.Action = Find
		c.getRelatedResources(w, ctx)
	case jsonapi.GetRelationship:
		ctx.Action = Find
		c.getRelationship(w, ctx)
	case jsonapi.SetRelationship:
		ctx.Action = Update
		c.setRelationship(w, ctx, doc)
	case jsonapi.AppendToRelationship:
		ctx.Action = Update
		c.appendToRelationship(w, ctx, doc)
	case jsonapi.RemoveFromRelationship:
		ctx.Action = Update
		c.removeFromRelationship(w, ctx, doc)
	}
}

func (c *Controller) listResources(w http.ResponseWriter, ctx *Context) {
	// prepare query
	ctx.Query = bson.M{}

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
	stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, ctx.Response))
}

func (c *Controller) findResource(w http.ResponseWriter, ctx *Context) {
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
	stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, ctx.Response))
}

func (c *Controller) createResource(w http.ResponseWriter, ctx *Context, doc *jsonapi.Document) {
	// basic input data check
	if doc.Data.One == nil {
		stack.Abort(jsonapi.BadRequest("Resource object expected"))
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
	stack.AbortIf(ctx.Store.C(ctx.Model).Insert(ctx.Model))

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
	stack.AbortIf(jsonapi.WriteResponse(w, http.StatusCreated, ctx.Response))
}

func (c *Controller) updateResource(w http.ResponseWriter, ctx *Context, doc *jsonapi.Document) {
	// basic input data check
	if doc.Data.One == nil {
		stack.Abort(jsonapi.BadRequest("Resource object expected"))
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
	stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, ctx.Response))
}

func (c *Controller) deleteResource(w http.ResponseWriter, ctx *Context) {
	// load model
	c.loadModel(ctx)

	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// query db
	stack.AbortIf(ctx.Store.C(c.Model).Remove(ctx.Query))

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// set status
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) getRelatedResources(w http.ResponseWriter, ctx *Context) {
	// load model
	c.loadModel(ctx)

	// prepare resource type
	var relationField *Field

	// find requested relationship
	for _, field := range ctx.Model.Meta().Fields {
		if field.RelName == ctx.JSONAPIRequest.RelatedResource {
			relationField = &field
			break
		}
	}

	// check resource type
	if relationField == nil {
		stack.Abort(jsonapi.BadRequest("Relationship does not exist"))
	}

	// get related controller
	pluralName := relationField.RelType
	relatedController := ctx.Group.controllers[pluralName]

	// check related controller
	if relatedController == nil {
		stack.Abort(fmt.Errorf("Missing related controller for %s", pluralName))
	}

	// copy context and request
	newCtx := &Context{
		Store: ctx.Store,
		JSONAPIRequest: &jsonapi.Request{
			ResourceType: pluralName,
			Include:      ctx.JSONAPIRequest.Include,
			PageNumber:   ctx.JSONAPIRequest.PageNumber,
			PageSize:     ctx.JSONAPIRequest.PageSize,
			Sorting:      ctx.JSONAPIRequest.Sorting,
			Fields:       ctx.JSONAPIRequest.Fields,
			Filters:      ctx.JSONAPIRequest.Filters,
		},
		HTTPRequest: ctx.HTTPRequest,
		Controller:  relatedController,
		Group:       ctx.Group,
	}

	// finish to one relationship
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
		newCtx.Action = Find
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
		stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, newCtx.Response))
	}

	// finish to many relationship
	if relationField.ToMany {
		// get ids from loaded model
		ids := ctx.Model.MustGet(relationField.Name).([]bson.ObjectId)

		// tweak context
		newCtx.Action = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set query
		newCtx.Query = bson.M{
			"_id": bson.M{
				"$in": ids,
			},
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
		stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, newCtx.Response))
	}

	// finish has many relationship
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
			stack.Abort(fmt.Errorf("No relationship matching the inverse name %s", relationField.RelInverse))
		}

		// tweak context
		newCtx.Action = List
		newCtx.JSONAPIRequest.Intent = jsonapi.ListResources

		// set query
		newCtx.Query = bson.M{
			filterName: bson.M{
				"$in": []bson.ObjectId{ctx.Model.ID()},
			},
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
		stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, newCtx.Response))
	}
}

func (c *Controller) getRelationship(w http.ResponseWriter, ctx *Context) {
	// load model
	c.loadModel(ctx)

	// get resource
	resource := c.resourceForModel(ctx, ctx.Model)

	// get relationship
	ctx.Response = resource.Relationships[ctx.JSONAPIRequest.Relationship]

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	stack.AbortIf(jsonapi.WriteResponse(w, http.StatusOK, ctx.Response))
}

func (c *Controller) setRelationship(w http.ResponseWriter, ctx *Context, doc *jsonapi.Document) {
	// load model
	c.loadModel(ctx)

	// assign relationship
	c.assignRelationship(ctx, ctx.JSONAPIRequest.Relationship, doc)

	// save model
	c.updateModel(ctx)

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) appendToRelationship(w http.ResponseWriter, ctx *Context, doc *jsonapi.Document) {
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
				stack.Abort(jsonapi.BadRequest("Invalid relationship ID"))
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

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) removeFromRelationship(w http.ResponseWriter, ctx *Context, doc *jsonapi.Document) {
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
				stack.Abort(jsonapi.BadRequest("Invalid relationship ID"))
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

	// run notifiers
	c.runCallbacks(c.Notifiers, ctx, http.StatusInternalServerError)

	// write result
	w.WriteHeader(http.StatusNoContent)
}

func (c *Controller) loadModel(ctx *Context) {
	// validate id
	if !bson.IsObjectIdHex(ctx.JSONAPIRequest.ResourceID) {
		stack.Abort(jsonapi.BadRequest("Invalid resource ID"))
	}

	// prepare context
	ctx.Query = bson.M{
		"_id": bson.ObjectIdHex(ctx.JSONAPIRequest.ResourceID),
	}

	// run authorizers
	c.runCallbacks(c.Authorizers, ctx, http.StatusUnauthorized)

	// prepare object
	obj := c.Model.Meta().Make()

	// query db
	err := ctx.Store.C(c.Model).Find(ctx.Query).One(obj)
	if err == mgo.ErrNotFound {
		stack.Abort(jsonapi.NotFound("Resource not found"))
	}
	stack.AbortIf(err)

	// initialize and set model
	ctx.Model = Init(obj.(Model))
}

func (c *Controller) loadModels(ctx *Context) []Model {
	// add filters
	for _, filter := range c.Filters {
		field := c.Model.Meta().MustFindField(filter)

		// check if filter is present
		if values, ok := ctx.JSONAPIRequest.Filters[field.JSONName]; ok {
			if field.Kind == reflect.Bool && len(values) == 1 {
				ctx.Query[field.BSONName] = values[0] == "true"
				continue
			}

			ctx.Query[field.BSONName] = bson.M{"$in": values}
		}
	}

	// add sorting
	for _, params := range ctx.JSONAPIRequest.Sorting {
		for _, sorter := range c.Sorters {
			field := c.Model.Meta().MustFindField(sorter)

			// check if sorting is present
			if params == field.JSONName || params == "-"+field.JSONName {
				ctx.Sorting = append(ctx.Sorting, params)
			}
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
	query := ctx.Store.C(c.Model).Find(ctx.Query).Sort(ctx.Sorting...)

	// add pagination
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		query = query.Limit(ctx.JSONAPIRequest.PageSize).Skip((ctx.JSONAPIRequest.PageNumber - 1) * ctx.JSONAPIRequest.PageSize)
	}

	// query db
	err := query.All(slicePtr)
	stack.AbortIf(err)

	// init all models in slice
	return InitSlice(slicePtr)
}

func (c *Controller) assignData(ctx *Context, res *jsonapi.Resource) {
	// map attributes to struct
	stack.AbortIf(res.Attributes.Assign(ctx.Model))

	// iterate relationships
	for name, rel := range res.Relationships {
		c.assignRelationship(ctx, name, rel)
	}
}

func (c *Controller) assignRelationship(ctx *Context, name string, rel *jsonapi.Document) {
	// assign relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field matches relationship
		if field.RelName != name || (!field.ToOne && !field.ToMany) {
			continue
		}

		// handle to one relationship
		if field.ToOne {
			// prepare zero value
			var id bson.ObjectId

			// set and check id if available
			if rel.Data != nil && rel.Data.One != nil {
				id = bson.ObjectIdHex(rel.Data.One.ID)

				// return error for an invalid id
				if !id.Valid() {
					stack.Abort(jsonapi.BadRequest("Invalid relationship ID"))
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

		// handle to many relationship
		if field.ToMany {
			// prepare slice of ids
			ids := make([]bson.ObjectId, len(rel.Data.Many))

			// range over all resources
			for i, r := range rel.Data.Many {
				// set id
				ids[i] = bson.ObjectIdHex(r.ID)

				// return error for an invalid id
				if !ids[i].Valid() {
					stack.Abort(jsonapi.BadRequest("Invalid relationship ID"))
				}
			}

			// set ids
			ctx.Model.MustSet(field.Name, ids)
		}
	}
}

func (c *Controller) updateModel(ctx *Context) {
	// run validators
	c.runCallbacks(c.Validators, ctx, http.StatusBadRequest)

	// update model
	stack.AbortIf(ctx.Store.C(ctx.Model).Update(ctx.Query, ctx.Model))
}

func (c *Controller) resourceForModel(ctx *Context, model Model) *jsonapi.Resource {
	// create map from model
	m, err := jsonapi.StructToMap(model, ctx.JSONAPIRequest.Fields[model.Meta().PluralName])
	stack.AbortIf(err)

	// prepare resource
	resource := &jsonapi.Resource{
		Type:          model.Meta().PluralName,
		ID:            model.ID().Hex(),
		Attributes:    m,
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := ctx.JSONAPIRequest.Prefix + "/" + model.Meta().PluralName + "/" + model.ID().Hex()

	// go through all relationships
	for _, field := range model.Meta().Fields {
		// check if relationship
		if !field.ToOne && !field.ToMany && !field.HasMany {
			continue
		}

		// prepare relationship links
		links := &jsonapi.DocumentLinks{
			Self:    base + "/relationships/" + field.RelName,
			Related: base + "/" + field.RelName,
		}

		// handle to one relationship
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
				stack.Abort(fmt.Errorf("No relationship matching the inverse name %s", field.RelInverse))
			}

			// TODO: We should run the related controllers authenticator.

			// load all referenced ids
			var ids []bson.ObjectId
			err := ctx.Store.C(relatedController.Model).Find(bson.M{
				filterName: bson.M{
					"$in": []bson.ObjectId{model.ID()},
				},
			}).Distinct("_id", &ids)
			stack.AbortIf(err)

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

	return resource
}

func (c *Controller) resourcesForModels(ctx *Context, models []Model) []*jsonapi.Resource {
	// prepare resources
	resources := make([]*jsonapi.Resource, 0, len(models))

	// create resources
	for _, model := range models {
		resources = append(resources, c.resourceForModel(ctx, model))
	}

	return resources
}

func (c *Controller) listLinks(self string, ctx *Context) *jsonapi.DocumentLinks {
	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: self,
	}

	// add pagination links
	if ctx.JSONAPIRequest.PageNumber > 0 && ctx.JSONAPIRequest.PageSize > 0 {
		// get total amount of resources
		n, err := c.Store.C(c.Model).Find(ctx.Query).Count()
		stack.AbortIf(err)

		// calculate last page
		lastPage := int(math.Ceil(float64(n) / float64(ctx.JSONAPIRequest.PageSize)))

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

func (c *Controller) runCallbacks(list []Callback, ctx *Context, errorStatus int) {
	// run callbacks and handle errors
	for _, cb := range list {
		err := cb(ctx)
		if isFatal(err) {
			stack.Abort(err)
		} else if err != nil {
			stack.Abort(&jsonapi.Error{
				Status: errorStatus,
				Detail: err.Error(),
			})
		}
	}
}
