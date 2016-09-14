// Package jsonapi implements components to build JSON APIs with fire.
package jsonapi

import (
	"fmt"
	"math"
	"net/http"
	"reflect"

	"github.com/gonfire/fire/model"
	"github.com/gonfire/jsonapi"
	"github.com/gonfire/jsonapi/adapter"
	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// A Controller provides a JSON API based interface to a model.
//
// Note: Controllers must not be modified after adding to an application.
type Controller struct {
	// The model that this controller should provide (e.g. &Foo{}).
	Model model.Model

	// The store that is used to retrieve and persist the model.
	Store *model.Store

	// The Authorizer is run on all actions. Will return an Unauthorized status
	// if an user error is returned.
	Authorizer Callback

	// The Validator is run to validate Create, Update and Delete actions. Will
	// return a Bad Request status if an user error is returned.
	Validator Callback

	// The NoList property can be set to true if the resource is only listed
	// through relationships from other resources. This is useful for
	// resources like comments that should never listed without a relationship.
	NoList bool

	// The ListLimit can be set to a value higher than 1 to enforce paginated
	// responses and restrain the page size to be within one and the limit.
	ListLimit int

	group *Group
}

func (c *Controller) register(router *echo.Echo, prefix string) {
	pluralName := c.Model.Meta().PluralName

	// add basic operations
	router.GET(prefix+"/"+pluralName, c.generalHandler)
	router.POST(prefix+"/"+pluralName, c.generalHandler)
	router.GET(prefix+"/"+pluralName+"/:id", c.generalHandler)
	router.PATCH(prefix+"/"+pluralName+"/:id", c.generalHandler)
	router.DELETE(prefix+"/"+pluralName+"/:id", c.generalHandler)

	// process all relationships
	for _, field := range c.Model.Meta().Fields {
		// skip if empty
		if field.RelName == "" {
			continue
		}

		// get name
		name := field.RelName

		// add relationship queries
		router.GET(prefix+"/"+pluralName+"/:id/"+name, c.generalHandler)
		router.GET(prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)

		// add relationship management operations
		if field.ToOne || field.ToMany {
			router.PATCH(prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)
		}
		if field.ToMany {
			router.POST(prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)
			router.DELETE(prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)
		}
	}
}

func (c *Controller) generalHandler(e echo.Context) error {
	r := adapter.BridgeRequest(e.Request())
	w := adapter.BridgeResponse(e.Response())

	// parse incoming JSON API request
	req, err := jsonapi.ParseRequest(r, c.group.prefix)
	if err != nil {
		return jsonapi.WriteError(w, err)
	}

	// handle no list setting
	if req.Intent == jsonapi.ListResources && c.NoList {
		return jsonapi.WriteError(w, jsonapi.ErrorFromStatus(
			http.StatusMethodNotAllowed,
			"Listing ist disabled for this resource.",
		))
	}

	// parse body if available
	var doc *jsonapi.Document
	if req.Intent.DocumentExpected() {
		doc, err = jsonapi.ParseDocument(e.Request().Body())
		if err != nil {
			return jsonapi.WriteError(w, err)
		}
	}

	// copy store
	store := c.Store.Copy()

	// ensure store will be closed
	defer store.Close()

	// prepare context
	var ctx *Context

	// call specific handlers based on the request intent
	switch req.Intent {
	case jsonapi.ListResources:
		ctx = buildContext(store, List, req, e)
		err = c.listResources(ctx)
	case jsonapi.FindResource:
		ctx = buildContext(store, Find, req, e)
		err = c.findResource(ctx)
	case jsonapi.CreateResource:
		ctx = buildContext(store, Create, req, e)
		err = c.createResource(ctx, doc)
	case jsonapi.UpdateResource:
		ctx = buildContext(store, Update, req, e)
		err = c.updateResource(ctx, doc)
	case jsonapi.DeleteResource:
		ctx = buildContext(store, Delete, req, e)
		err = c.deleteResource(ctx)
	case jsonapi.GetRelatedResources:
		ctx = buildContext(store, Find, req, e)
		err = c.getRelatedResources(ctx)
	case jsonapi.GetRelationship:
		ctx = buildContext(store, Find, req, e)
		err = c.getRelationship(ctx)
	case jsonapi.SetRelationship:
		ctx = buildContext(store, Update, req, e)
		err = c.setRelationship(ctx, doc)
	case jsonapi.AppendToRelationship:
		ctx = buildContext(store, Update, req, e)
		err = c.appendToRelationship(ctx, doc)
	case jsonapi.RemoveFromRelationship:
		ctx = buildContext(store, Update, req, e)
		err = c.removeFromRelationship(ctx, doc)
	}

	// write any left over errors
	if err != nil {
		return jsonapi.WriteError(w, err)
	}

	return nil
}

func (c *Controller) listResources(ctx *Context) error {
	w := adapter.BridgeResponse(ctx.Echo.Response())

	// prepare query
	ctx.Query = bson.M{}

	// load models
	slice, err := c.loadModels(ctx)
	if err != nil {
		return err
	}

	// get resources
	resources, err := c.resourcesForSlice(ctx, slice)
	if err != nil {
		return err
	}

	// get list links
	links, err := c.listLinks(ctx.Request.Self(), ctx)
	if err != nil {
		return err
	}

	// write result
	return jsonapi.WriteResources(w, http.StatusOK, resources, links)
}

func (c *Controller) findResource(ctx *Context) error {
	w := adapter.BridgeResponse(ctx.Echo.Response())

	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx, ctx.Model)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self(),
	}

	// write result
	return jsonapi.WriteResource(w, http.StatusOK, resource, links)
}

func (c *Controller) createResource(ctx *Context, doc *jsonapi.Document) error {
	w := adapter.BridgeResponse(ctx.Echo.Response())

	// basic input data check
	if doc.Data.One == nil {
		return jsonapi.BadRequest("Resource object expected")
	}

	// create new model
	ctx.Model = c.Model.Meta().Make()

	// assign attributes
	err := c.assignData(ctx, doc.Data.One)
	if err != nil {
		return err
	}

	// run authorizer if available
	err = c.runCallback(c.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// validate model
	err = ctx.Model.Validate(true)
	if err != nil {
		return jsonapi.BadRequest(err.Error())
	}

	// run validator if available
	err = c.runCallback(c.Validator, ctx, http.StatusBadRequest)
	if err != nil {
		return err
	}

	// insert model
	err = ctx.Store.C(ctx.Model).Insert(ctx.Model)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx, ctx.Model)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self() + "/" + ctx.Model.ID().Hex(),
	}

	// write result
	return jsonapi.WriteResource(w, http.StatusCreated, resource, links)
}

func (c *Controller) updateResource(ctx *Context, doc *jsonapi.Document) error {
	w := adapter.BridgeResponse(ctx.Echo.Response())

	// basic input data check
	if doc.Data.One == nil {
		return jsonapi.BadRequest("Resource object expected")
	}

	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// assign attributes
	err = c.assignData(ctx, doc.Data.One)
	if err != nil {
		return err
	}

	// save model
	err = c.saveModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx, ctx.Model)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self(),
	}

	// write result
	return jsonapi.WriteResource(w, http.StatusOK, resource, links)
}

func (c *Controller) deleteResource(ctx *Context) error {
	// validate id
	if !bson.IsObjectIdHex(ctx.Request.ResourceID) {
		return jsonapi.BadRequest("Invalid ID")
	}

	// prepare context
	ctx.Query = bson.M{
		"_id": bson.ObjectIdHex(ctx.Request.ResourceID),
	}

	// run authorizer if available
	if err := c.runCallback(c.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return err
	}

	// run validator if available
	if err := c.runCallback(c.Validator, ctx, http.StatusBadRequest); err != nil {
		return err
	}

	// query db
	err := ctx.Store.C(c.Model).Remove(ctx.Query)
	if err != nil {
		return err
	}

	// set status
	ctx.Echo.Response().WriteHeader(http.StatusNoContent)

	return nil
}

func (c *Controller) getRelatedResources(ctx *Context) error {
	w := adapter.BridgeResponse(ctx.Echo.Response())

	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// prepare resource type
	var relationField *model.Field

	// find requested relationship
	for _, field := range c.Model.Meta().Fields {
		if field.RelName == ctx.Request.RelatedResource {
			relationField = &field
			break
		}
	}

	// check resource type
	if relationField == nil {
		return jsonapi.BadRequest("Relationship does not exist")
	}

	// get related controller
	pluralName := relationField.RelType
	relatedController := c.group.controllers[pluralName]

	// check related controller
	if relatedController == nil {
		panic("Missing related controller for " + pluralName)
	}

	// copy context and request
	newCtx := &Context{
		Request: &jsonapi.Request{
			ResourceType: pluralName,
			Include:      ctx.Request.Include,
			PageNumber:   ctx.Request.PageNumber,
			PageSize:     ctx.Request.PageSize,
			Sorting:      ctx.Request.Sorting,
			Fields:       ctx.Request.Fields,
			Filters:      ctx.Request.Filters,
		},
		Store: ctx.Store,
		Echo:  ctx.Echo,
	}

	// finish to one relationship
	if relationField.ToOne {
		// prepare id
		var id string

		// prepare links
		links := &jsonapi.DocumentLinks{
			Self: ctx.Request.Self(),
		}

		// lookup id of related resource
		if relationField.Optional {
			// lookup optional id on loaded model
			oid := ctx.Model.Get(relationField.Name).(*bson.ObjectId)

			// TODO: Test present optional id.

			// check if missing
			if oid != nil {
				id = oid.Hex()
			} else {
				// write empty response
				return jsonapi.WriteResource(w, http.StatusOK, nil, links)
			}
		} else {
			// lookup id on loaded model
			id = ctx.Model.Get(relationField.Name).(bson.ObjectId).Hex()
		}

		// tweak context
		newCtx.Action = Find
		newCtx.Request.Intent = jsonapi.FindResource
		newCtx.Request.ResourceID = id

		// load model
		err := relatedController.loadModel(newCtx)
		if err != nil {
			return err
		}

		// get resource
		resource, err := relatedController.resourceForModel(newCtx, newCtx.Model)
		if err != nil {
			return err
		}

		// write result
		return jsonapi.WriteResource(w, http.StatusOK, resource, links)
	}

	// finish to many relationship
	if relationField.ToMany {
		// get ids from loaded model
		ids := ctx.Model.Get(relationField.Name).([]bson.ObjectId)

		// tweak context
		newCtx.Action = List
		newCtx.Request.Intent = jsonapi.ListResources

		// set query
		newCtx.Query = bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		}

		// load related models
		slice, err := relatedController.loadModels(newCtx)
		if err != nil {
			return err
		}

		// get related resources
		resources, err := relatedController.resourcesForSlice(newCtx, slice)
		if err != nil {
			return err
		}

		// get list links
		links, err := relatedController.listLinks(ctx.Request.Self(), newCtx)
		if err != nil {
			return err
		}

		// write result
		return jsonapi.WriteResources(w, http.StatusOK, resources, links)
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
			return fmt.Errorf("no relationship matching the inverse name %s", relationField.RelInverse)
		}

		// tweak context
		newCtx.Action = List
		newCtx.Request.Intent = jsonapi.ListResources

		// set query
		newCtx.Query = bson.M{
			filterName: bson.M{
				"$in": []bson.ObjectId{ctx.Model.ID()},
			},
		}

		// load related models
		slice, err := relatedController.loadModels(newCtx)
		if err != nil {
			return err
		}

		// get related resources
		resources, err := relatedController.resourcesForSlice(newCtx, slice)
		if err != nil {
			return err
		}

		// get list links
		links, err := relatedController.listLinks(ctx.Request.Self(), newCtx)
		if err != nil {
			return err
		}

		// write result
		return jsonapi.WriteResources(w, http.StatusOK, resources, links)
	}

	return nil
}

func (c *Controller) getRelationship(ctx *Context) error {
	w := adapter.BridgeResponse(ctx.Echo.Response())

	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx, ctx.Model)
	if err != nil {
		return err
	}

	// get relationship
	relationship := resource.Relationships[ctx.Request.Relationship]

	// write result
	return jsonapi.WriteResponse(w, http.StatusOK, relationship)
}

func (c *Controller) setRelationship(ctx *Context, doc *jsonapi.Document) error {
	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// assign relationship
	err = c.assignRelationship(ctx, ctx.Request.Relationship, doc)
	if err != nil {
		return err
	}

	// save model
	err = c.saveModel(ctx)
	if err != nil {
		return err
	}

	// write result
	ctx.Echo.Response().WriteHeader(http.StatusNoContent)
	return nil
}

func (c *Controller) appendToRelationship(ctx *Context, doc *jsonapi.Document) error {
	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// append relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field matches relationship
		if !field.ToMany || field.RelName != ctx.Request.Relationship {
			continue
		}

		// process all references
		for _, ref := range doc.Data.Many {
			// get id
			refID := bson.ObjectIdHex(ref.ID)

			// return error for an invalid id
			if !refID.Valid() {
				return jsonapi.BadRequest("Invalid relationship ID")
			}

			// prepare mark
			var present bool

			// get current ids
			ids := ctx.Model.Get(field.Name).([]bson.ObjectId)

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
				ctx.Model.Set(field.Name, ids)
			}
		}
	}

	// save model
	err = c.saveModel(ctx)
	if err != nil {
		return err
	}

	// write result
	ctx.Echo.Response().WriteHeader(http.StatusNoContent)
	return nil
}

func (c *Controller) removeFromRelationship(ctx *Context, doc *jsonapi.Document) error {
	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// remove relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field matches relationship
		if !field.ToMany || field.RelName != ctx.Request.Relationship {
			continue
		}

		// process all references
		for _, ref := range doc.Data.Many {
			// get id
			refID := bson.ObjectIdHex(ref.ID)

			// return error for an invalid id
			if !refID.Valid() {
				return jsonapi.BadRequest("Invalid relationship ID")
			}

			// prepare mark
			var pos = -1

			// get current ids
			ids := ctx.Model.Get(field.Name).([]bson.ObjectId)

			// check if id is already present
			for i, id := range ids {
				if id == refID {
					pos = i
				}
			}

			// remove id if present
			if pos >= 0 {
				ids = append(ids[:pos], ids[pos+1:]...)
				ctx.Model.Set(field.Name, ids)
			}
		}
	}

	// save model
	err = c.saveModel(ctx)
	if err != nil {
		return err
	}

	// write result
	ctx.Echo.Response().WriteHeader(http.StatusNoContent)
	return nil
}

func (c *Controller) runCallback(cb Callback, ctx *Context, errorStatus int) error {
	// check if callback is available
	if cb == nil {
		return nil
	}

	// run callback and handle errors
	err := cb(ctx)
	if isFatal(err) {
		return err
	} else if err != nil {
		// return user error
		return &jsonapi.Error{
			Status: errorStatus,
			Detail: err.Error(),
		}
	}

	return nil
}

func (c *Controller) loadModel(ctx *Context) error {
	// validate id
	if !bson.IsObjectIdHex(ctx.Request.ResourceID) {
		return jsonapi.BadRequest("Invalid resource ID")
	}

	// prepare context
	ctx.Query = bson.M{
		"_id": bson.ObjectIdHex(ctx.Request.ResourceID),
	}

	// run authorizer if available
	err := c.runCallback(c.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// prepare object
	obj := c.Model.Meta().Make()

	// query db
	err = ctx.Store.C(c.Model).Find(ctx.Query).One(obj)
	if err == mgo.ErrNotFound {
		return jsonapi.NotFound("Resource not found")
	} else if err != nil {
		return err
	}

	// initialize and set model
	ctx.Model = model.Init(obj.(model.Model))

	return nil
}

func (c *Controller) loadModels(ctx *Context) (interface{}, error) {
	// add filters
	for _, field := range c.Model.Meta().Fields {
		if field.Filterable {
			if values, ok := ctx.Request.Filters[field.JSONName]; ok {
				if field.Type == reflect.Bool && len(values) == 1 {
					ctx.Query[field.BSONName] = values[0] == "true"
					continue
				}

				ctx.Query[field.BSONName] = bson.M{"$in": values}
			}
		}
	}

	// add sorting
	for _, params := range ctx.Request.Sorting {
		for _, field := range c.Model.Meta().Fields {
			if field.Sortable {
				if params == field.BSONName || params == "-"+field.BSONName {
					ctx.Sorting = append(ctx.Sorting, params)
				}
			}
		}
	}

	// honor list limit
	if c.ListLimit > 0 && (ctx.Request.PageSize == 0 || ctx.Request.PageSize > c.ListLimit) {
		// restrain page size
		ctx.Request.PageSize = c.ListLimit

		// enforce pagination
		if ctx.Request.PageNumber == 0 {
			ctx.Request.PageNumber = 1
		}
	}

	// run authorizer if available
	err := c.runCallback(c.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return nil, err
	}

	// prepare slice
	slicePtr := c.Model.Meta().MakeSlice()

	// prepare query
	query := ctx.Store.C(c.Model).Find(ctx.Query).Sort(ctx.Sorting...)

	// add pagination
	if ctx.Request.PageNumber > 0 && ctx.Request.PageSize > 0 {
		query = query.Limit(ctx.Request.PageSize).Skip((ctx.Request.PageNumber - 1) * ctx.Request.PageSize)
	}

	// query db
	err = query.All(slicePtr)
	if err != nil {
		return nil, err
	}

	// init all models in slice
	slice := reflect.ValueOf(slicePtr).Elem()
	for i := 0; i < slice.Len(); i++ {
		model.Init(slice.Index(i).Interface().(model.Model))
	}

	return slicePtr, nil
}

func (c *Controller) assignData(ctx *Context, res *jsonapi.Resource) error {
	// map attributes to struct
	err := res.Attributes.Assign(ctx.Model)
	if err != nil {
		return err
	}

	// iterate relationships
	for name, rel := range res.Relationships {
		err = c.assignRelationship(ctx, name, rel)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) assignRelationship(ctx *Context, name string, rel *jsonapi.Document) error {
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
					return jsonapi.BadRequest("Invalid relationship ID")
				}
			}

			// handle non optional field first
			if !field.Optional {
				ctx.Model.Set(field.Name, id)
				continue
			}

			// assign for a zero value optional field
			if id != "" {
				ctx.Model.Set(field.Name, &id)
			} else {
				var nilID *bson.ObjectId
				ctx.Model.Set(field.Name, nilID)
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
					return jsonapi.BadRequest("Invalid relationship ID")
				}
			}

			// set ids
			ctx.Model.Set(field.Name, ids)
		}
	}

	return nil
}

func (c *Controller) saveModel(ctx *Context) error {
	// validate model
	err := ctx.Model.Validate(false)
	if err != nil {
		return jsonapi.BadRequest(err.Error())
	}

	// run validator if available
	err = c.runCallback(c.Validator, ctx, http.StatusBadRequest)
	if err != nil {
		return err
	}

	// update model
	return ctx.Store.C(c.Model).Update(ctx.Query, ctx.Model)
}

func (c *Controller) resourceForModel(ctx *Context, model model.Model) (*jsonapi.Resource, error) {
	// prepare resource
	resource := &jsonapi.Resource{
		Type:          c.Model.Meta().PluralName,
		ID:            model.ID().Hex(),
		Attributes:    jsonapi.StructToMap(model, ctx.Request.Fields[c.Model.Meta().PluralName]),
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := c.group.prefix + "/" + c.Model.Meta().PluralName + "/" + model.ID().Hex()

	// TODO: Support included resources (one level).

	// go through all relationships
	for _, field := range model.Meta().Fields {
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
				oid := model.Get(field.Name).(*bson.ObjectId)

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
					ID:   model.Get(field.Name).(bson.ObjectId).Hex(),
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
			ids := model.Get(field.Name).([]bson.ObjectId)

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
			relatedController := c.group.controllers[field.RelType]

			// check existence
			if relatedController == nil {
				panic("missing related controller " + field.RelType)
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
				return nil, fmt.Errorf("no relationship matching the inverse name %s", field.RelInverse)
			}

			// load all referenced ids
			var ids []bson.ObjectId
			err := ctx.Store.C(relatedController.Model).Find(bson.M{
				filterName: bson.M{
					"$in": []bson.ObjectId{model.ID()},
				},
			}).Distinct("_id", &ids)
			if err != nil {
				return nil, err
			}

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

	return resource, nil
}

func (c *Controller) resourcesForSlice(ctx *Context, ptr interface{}) ([]*jsonapi.Resource, error) {
	// dereference pointer to slice
	slice := reflect.ValueOf(ptr).Elem()

	// prepare resources
	resources := make([]*jsonapi.Resource, 0, slice.Len())

	// create resources
	for i := 0; i < slice.Len(); i++ {
		resource, err := c.resourceForModel(ctx, slice.Index(i).Interface().(model.Model))
		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func (c *Controller) listLinks(self string, ctx *Context) (*jsonapi.DocumentLinks, error) {
	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: self,
	}

	// add pagination links
	if ctx.Request.PageNumber > 0 && ctx.Request.PageSize > 0 {
		// get total amount of resources
		n, err := c.Store.C(c.Model).Find(ctx.Query).Count()
		if err != nil {
			return nil, err
		}

		// calculate last page
		lastPage := int(math.Ceil(float64(n) / float64(ctx.Request.PageSize)))

		// add basic pagination links
		links.Self = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, ctx.Request.PageNumber, ctx.Request.PageSize)
		links.First = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, 1, ctx.Request.PageSize)
		links.Last = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, lastPage, ctx.Request.PageSize)

		// add previous link if not on first page
		if ctx.Request.PageNumber > 1 {
			links.Previous = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, ctx.Request.PageNumber-1, ctx.Request.PageSize)
		}

		// add next link if not on last page
		if ctx.Request.PageNumber < lastPage {
			links.Next = fmt.Sprintf("%s?page[number]=%d&page[size]=%d", self, ctx.Request.PageNumber+1, ctx.Request.PageSize)
		}
	}

	return links, nil
}
