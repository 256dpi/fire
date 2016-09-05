package fire

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gonfire/jsonapi"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// A Controller provides a JSON API based interface to a model.
//
// Note: Controllers must not be modified after adding to an Endpoint.
type Controller struct {
	// The model that this controller should provide (e.g. &Foo{}).
	Model Model

	// The Authorizer is run on all actions. Will return an Unauthorized status
	// if an user error is returned.
	Authorizer Callback

	// The Validator is run to validate Create, Update and Delete actions. Will
	// return a Bad Request status if an user error is returned.
	Validator Callback

	endpoint *Endpoint
}

func (c *Controller) generalHandler(gctx *gin.Context) {
	// parse incoming JSON API request
	req, err := jsonapi.ParseRequest(gctx.Request, c.endpoint.prefix)
	if err != nil {
		jsonapi.WriteError(gctx.Writer, err)
		gctx.Error(err)
		gctx.Abort()
		return
	}

	// parse body if available
	var doc *jsonapi.Document
	if req.Intent.DocumentExpected() {
		doc, err = jsonapi.ParseBody(gctx.Request.Body)
		if err != nil {
			jsonapi.WriteError(gctx.Writer, err)
			gctx.Error(err)
			gctx.Abort()
			return
		}
	}

	// call specific handlers based on the request intent
	switch req.Intent {
	case jsonapi.ListResources:
		ctx := c.buildContext(FindAll, req, gctx)
		err = c.listResources(ctx)
	case jsonapi.FindResource:
		ctx := c.buildContext(FindOne, req, gctx)
		err = c.findResource(ctx)
	case jsonapi.CreateResource:
		ctx := c.buildContext(Create, req, gctx)
		err = c.createResource(ctx, doc)
	case jsonapi.UpdateResource:
		ctx := c.buildContext(Update, req, gctx)
		err = c.updateResource(ctx, doc)
	case jsonapi.DeleteResource:
		ctx := c.buildContext(Delete, req, gctx)
		err = c.deleteResource(ctx)
	case jsonapi.GetRelatedResources:
		ctx := c.buildContext(0, req, gctx)
		err = c.getRelatedResources(ctx)
	case jsonapi.GetRelationship:
		ctx := c.buildContext(0, req, gctx)
		err = c.getRelationship(ctx)
	case jsonapi.SetRelationship:
		ctx := c.buildContext(Update, req, gctx)
		err = c.setRelationship(ctx, doc)
	case jsonapi.AppendToRelationship:
		ctx := c.buildContext(Update, req, gctx)
		err = c.appendToRelationship(ctx, doc)
	case jsonapi.RemoveFromRelationship:
		ctx := c.buildContext(Update, req, gctx)
		err = c.removeFromRelationship(ctx, doc)
	}

	// write any occurring errors
	if err != nil {
		jsonapi.WriteError(gctx.Writer, err)
		gctx.Error(err)
		gctx.Abort()
	}
}

func (c *Controller) listResources(ctx *Context) error {
	// prepare query
	ctx.Query = bson.M{}

	// load models
	err := c.loadModels(ctx)
	if err != nil {
		return err
	}

	// get resources
	resources, err := c.resourcesForSlice(ctx.slice)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self(),
	}

	// write result
	return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
}

func (c *Controller) findResource(ctx *Context) error {
	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx.Model)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self(),
	}

	// write result
	return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, resource, links)
}

func (c *Controller) createResource(ctx *Context, doc *jsonapi.Document) error {
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

	// query db
	err = c.endpoint.db.C(c.Model.Meta().Collection).Insert(ctx.Model)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx.Model)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self() + "/" + ctx.Model.ID().Hex(),
	}

	// write result
	return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusCreated, resource, links)
}

func (c *Controller) updateResource(ctx *Context, doc *jsonapi.Document) error {
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
	resource, err := c.resourceForModel(ctx.Model)
	if err != nil {
		return err
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self(),
	}

	// write result
	return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, resource, links)
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
	err := c.endpoint.db.C(c.Model.Meta().Collection).Remove(ctx.Query)
	if err != nil {
		return err
	}

	// set status
	ctx.GinContext.Status(http.StatusNoContent)

	return nil
}

func (c *Controller) getRelatedResources(ctx *Context) error {
	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// prepare resource type
	var relationField *Field

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

	// get related resource
	pluralName := relationField.RelType
	singularName := c.endpoint.nameMap[pluralName]
	relatedController := c.endpoint.controllerMap[singularName]

	// check related controller
	if relatedController == nil {
		return fmt.Errorf("related controller %s not found in map", pluralName)
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: ctx.Request.Self(),
	}

	// zero request
	ctx.Request.Intent = 0
	ctx.Request.ResourceType = pluralName
	ctx.Request.ResourceID = ""
	ctx.Request.RelatedResource = ""

	// finish to one relationship
	if relationField.ToOne {
		// prepare id
		var id string

		// handle optional field
		if relationField.Optional {
			// lookup id
			oid := ctx.Model.Get(relationField.Name).(*bson.ObjectId)

			// check if missing
			if oid != nil {
				id = oid.Hex()
			} else {
				// write empty response
				return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, nil, links)
			}
		} else {
			id = ctx.Model.Get(relationField.Name).(bson.ObjectId).Hex()
		}

		// modify context
		ctx.Request.Intent = jsonapi.FindResource
		ctx.Request.ResourceID = id

		// create new request
		ctx2 := relatedController.buildContext(FindOne, ctx.Request, ctx.GinContext)

		// load model
		err := relatedController.loadModel(ctx2)
		if err != nil {
			return err
		}

		// get resource
		resource, err := relatedController.resourceForModel(ctx2.Model)
		if err != nil {
			return err
		}

		// write result
		return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, resource, links)
	}

	// finish to many relationship
	if relationField.ToMany {
		// get ids
		ids := ctx.Model.Get(relationField.Name).([]bson.ObjectId)

		// modify context
		ctx.Request.Intent = jsonapi.ListResources

		// create new request
		ctx2 := relatedController.buildContext(FindAll, ctx.Request, ctx.GinContext)

		// set query
		ctx2.Query = bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		}

		// load related models
		err := relatedController.loadModels(ctx2)
		if err != nil {
			return err
		}

		// get related resources
		resources, err := relatedController.resourcesForSlice(ctx2.slice)
		if err != nil {
			return err
		}

		// write result
		return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
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

		// modify context
		ctx.Request.Intent = jsonapi.ListResources

		// create new request
		ctx2 := relatedController.buildContext(FindAll, ctx.Request, ctx.GinContext)

		// set query
		ctx2.Query = bson.M{
			filterName: bson.M{
				"$in": []bson.ObjectId{ctx.Model.ID()},
			},
		}

		// load related models
		err := relatedController.loadModels(ctx2)
		if err != nil {
			return err
		}

		// get related resources
		resources, err := relatedController.resourcesForSlice(ctx2.slice)
		if err != nil {
			return err
		}

		// write result
		return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
	}

	return nil
}

func (c *Controller) getRelationship(ctx *Context) error {
	// load model
	err := c.loadModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource, err := c.resourceForModel(ctx.Model)
	if err != nil {
		return err
	}

	// get relationship
	relationship := resource.Relationships[ctx.Request.Relationship]

	// write result
	return jsonapi.WriteResponse(ctx.GinContext.Writer, http.StatusOK, relationship)
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
	ctx.GinContext.Status(http.StatusNoContent)
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
		if field.RelName != ctx.Request.Relationship {
			continue
		}

		// check if field is a to many relationship
		if !field.ToMany {
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
	ctx.GinContext.Status(http.StatusNoContent)
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
		if field.RelName != ctx.Request.Relationship {
			continue
		}

		// check if field is a to many relationship
		if !field.ToMany {
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
	ctx.GinContext.Status(http.StatusNoContent)
	return nil
}

func (c *Controller) buildContext(action Action, req *jsonapi.Request, gctx *gin.Context) *Context {
	return &Context{
		Action:     action,
		DB:         c.endpoint.db,
		Request:    req,
		GinContext: gctx,
	}
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
	err = c.endpoint.db.C(c.Model.Meta().Collection).Find(ctx.Query).One(obj)
	if err == mgo.ErrNotFound {
		return jsonapi.NotFound("Resource not found")
	} else if err != nil {
		return err
	}

	// initialize and set model
	ctx.Model = Init(obj.(Model))

	return nil
}

func (c *Controller) loadModels(ctx *Context) error {
	// add filters
	for _, field := range c.Model.Meta().FieldsByTag("filterable") {
		if values, ok := ctx.Request.Filters[field.JSONName]; ok {
			if field.Type == reflect.Bool && len(values) == 1 {
				ctx.Query[field.BSONName] = values[0] == "true"
				continue
			}

			ctx.Query[field.BSONName] = bson.M{"$in": values}
		}
	}

	// add sorting
	for _, params := range ctx.Request.Sorting {
		for _, field := range c.Model.Meta().FieldsByTag("sortable") {
			if params == field.BSONName || params == "-"+field.BSONName {
				ctx.Sorting = append(ctx.Sorting, params)
			}
		}
	}

	// TODO: Enforce pagination automatically (20 items per page).

	// run authorizer if available
	err := c.runCallback(c.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// prepare slice
	ctx.slice = c.Model.Meta().MakeSlice()

	// query db
	err = c.endpoint.db.C(c.Model.Meta().Collection).Find(ctx.Query).
		Sort(ctx.Sorting...).All(ctx.slice)
	if err != nil {
		return err
	}

	// initialize slice
	InitSlice(ctx.slice)

	return nil
}

func (c *Controller) assignData(ctx *Context, res *jsonapi.Resource) error {
	// map attributes to struct
	err := jsonapi.MapToStruct(res.Attributes, ctx.Model)
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
		if field.RelName != name {
			continue
		}

		// check if field is a relationship
		if !field.ToOne && !field.ToMany {
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
	return c.endpoint.db.C(c.Model.Meta().Collection).Update(ctx.Query, ctx.Model)
}

func (c *Controller) resourceForModel(model Model) (*jsonapi.Resource, error) {
	// prepare resource
	resource := &jsonapi.Resource{
		Type:          c.Model.Meta().PluralName,
		ID:            model.ID().Hex(),
		Attributes:    model,
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := c.endpoint.prefix + "/" + c.Model.Meta().PluralName + "/" + model.ID().Hex()

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
			// get related resource
			singularName := c.endpoint.nameMap[field.RelType]
			relatedController := c.endpoint.controllerMap[singularName]

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
			err := c.endpoint.db.C(relatedController.Model.Meta().Collection).Find(bson.M{
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

func (c *Controller) resourcesForSlice(ptr interface{}) ([]*jsonapi.Resource, error) {
	// dereference pointer to slice
	slice := reflect.ValueOf(ptr).Elem()

	// prepare resources
	resources := make([]*jsonapi.Resource, 0, slice.Len())

	// create resource
	for i := 0; i < slice.Len(); i++ {
		_resources, err := c.resourceForModel(slice.Index(i).Interface().(Model))
		if err != nil {
			return nil, err
		}

		resources = append(resources, _resources)
	}

	return resources, nil
}
