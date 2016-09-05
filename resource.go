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

// A Resource provides an interface to a model.
//
// Note: Resources must not be modified after adding to an Endpoint.
type Resource struct {
	// The model that this resource should provide (e.g. &Foo{}).
	Model Model

	// The Authorizer is run on all actions. Will return an Unauthorized status
	// if an user error is returned.
	Authorizer Callback

	// The Validator is run to validate Create, Update and Delete actions. Will
	// return a Bad Request status if an user error is returned.
	Validator Callback

	endpoint *Endpoint
}

func (r *Resource) generalHandler(gctx *gin.Context) {
	// parse incoming JSON API request
	req, err := jsonapi.ParseRequest(gctx.Request, r.endpoint.prefix)
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
		ctx := r.buildContext(FindAll, req, gctx)
		err = r.listResources(ctx)
	case jsonapi.FindResource:
		ctx := r.buildContext(FindOne, req, gctx)
		err = r.findResource(ctx)
	case jsonapi.CreateResource:
		ctx := r.buildContext(Create, req, gctx)
		err = r.createResource(ctx, doc)
	case jsonapi.UpdateResource:
		ctx := r.buildContext(Update, req, gctx)
		err = r.updateResource(ctx, doc)
	case jsonapi.DeleteResource:
		ctx := r.buildContext(Delete, req, gctx)
		err = r.deleteResource(ctx)
	case jsonapi.GetRelatedResources:
		ctx := r.buildContext(0, req, gctx)
		err = r.getRelatedResources(ctx)
	case jsonapi.GetRelationship:
		ctx := r.buildContext(0, req, gctx)
		err = r.getRelationship(ctx)
	case jsonapi.SetRelationship:
		ctx := r.buildContext(Update, req, gctx)
		err = r.setRelationship(ctx, doc)
	case jsonapi.AppendToRelationship:
		ctx := r.buildContext(Update, req, gctx)
		err = r.appendToRelationship(ctx, doc)
	case jsonapi.RemoveFromRelationship:
		ctx := r.buildContext(Update, req, gctx)
		err = r.removeFromRelationship(ctx, doc)
	}

	// write any occurring errors
	if err != nil {
		jsonapi.WriteError(gctx.Writer, err)
		gctx.Error(err)
		gctx.Abort()
	}
}

func (r *Resource) listResources(ctx *Context) error {
	// prepare query
	ctx.Query = bson.M{}

	// load models
	err := r.loadModels(ctx)
	if err != nil {
		return err
	}

	// get resources
	resources := r.resourcesForSlice(ctx.slice)

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: r.endpoint.prefix + "/" + r.Model.Meta().PluralName,
	}

	// write result
	return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
}

func (r *Resource) findResource(ctx *Context) error {
	// load model
	err := r.loadModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource := r.resourceForModel(ctx.Model)

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: r.endpoint.prefix + "/" + r.Model.Meta().PluralName + "/" + ctx.Model.ID().Hex(),
	}

	// write result
	return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, resource, links)
}

func (r *Resource) createResource(ctx *Context, doc *jsonapi.Document) error {
	// basic input data check
	if doc.Data.One == nil {
		return jsonapi.BadRequest("Resource object expected")
	}

	// create new model
	ctx.Model = Init(r.Model.Meta().Make())

	// assign attributes
	err := r.assignData(ctx, doc.Data.One)
	if err != nil {
		return err
	}

	// run authorizer if available
	err = r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// validate model
	err = ctx.Model.Validate(true)
	if err != nil {
		return jsonapi.BadRequest(err.Error())
	}

	// run validator if available
	err = r.runCallback(r.Validator, ctx, http.StatusBadRequest)
	if err != nil {
		return err
	}

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Insert(ctx.Model)
	if err != nil {
		return err
	}

	// get resource
	resource := r.resourceForModel(ctx.Model)

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: r.endpoint.prefix + "/" + r.Model.Meta().PluralName + "/" + ctx.Model.ID().Hex(),
	}

	// write result
	return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusCreated, resource, links)
}

func (r *Resource) updateResource(ctx *Context, doc *jsonapi.Document) error {
	// basic input data check
	if doc.Data.One == nil {
		return jsonapi.BadRequest("Resource object expected")
	}

	// load model
	err := r.loadModel(ctx)
	if err != nil {
		return err
	}

	// assign attributes
	err = r.assignData(ctx, doc.Data.One)
	if err != nil {
		return err
	}

	// save model
	err = r.saveModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource := r.resourceForModel(ctx.Model)

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: r.endpoint.prefix + "/" + r.Model.Meta().PluralName + "/" + ctx.Model.ID().Hex(),
	}

	// write result
	return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, resource, links)
}

func (r *Resource) deleteResource(ctx *Context) error {
	// validate id
	if !bson.IsObjectIdHex(ctx.Request.ResourceID) {
		return jsonapi.BadRequest("Invalid ID")
	}

	// prepare context
	ctx.Query = bson.M{
		"_id": bson.ObjectIdHex(ctx.Request.ResourceID),
	}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return err
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return err
	}

	// query db
	err := r.endpoint.db.C(r.Model.Meta().Collection).Remove(ctx.Query)
	if err != nil {
		return err
	}

	// set status
	ctx.GinContext.Status(http.StatusNoContent)

	return nil
}

func (r *Resource) getRelatedResources(ctx *Context) error {
	// load model
	err := r.loadModel(ctx)
	if err != nil {
		return err
	}

	// prepare resource type
	var relationField *Field

	// find requested relationship
	for _, field := range r.Model.Meta().Fields {
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
	singularName := r.endpoint.nameMap[pluralName]
	resource := r.endpoint.resourceMap[singularName]

	// check resource
	if resource == nil {
		return fmt.Errorf("related resource %s not found in map", pluralName)
	}

	// generate base link
	base := r.endpoint.prefix + "/" + r.Model.Meta().PluralName + "/" + ctx.Model.ID().Hex() + "/" + ctx.Request.RelatedResource

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
				// TODO: What to do here?
			}
		} else {
			id = ctx.Model.Get(relationField.Name).(bson.ObjectId).Hex()
		}

		// modify context
		ctx.Request.Intent = jsonapi.FindResource
		ctx.Request.ResourceID = id

		// create new request
		ctx2 := resource.buildContext(FindOne, ctx.Request, ctx.GinContext)

		// load model
		err := resource.loadModel(ctx2)
		if err != nil {
			return err
		}

		// get resource
		_resource := resource.resourceForModel(ctx2.Model)

		// prepare links
		links := &jsonapi.DocumentLinks{
			Self: base,
		}

		// write result
		return jsonapi.WriteResource(ctx.GinContext.Writer, http.StatusOK, _resource, links)
	}

	// finish to many relationship
	if relationField.ToMany {
		// get ids
		ids := ctx.Model.Get(relationField.Name).([]bson.ObjectId)

		// modify context
		ctx.Request.Intent = jsonapi.ListResources

		// create new request
		ctx2 := resource.buildContext(FindAll, ctx.Request, ctx.GinContext)

		// set query
		ctx2.Query = bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		}

		// load related models
		err := resource.loadModels(ctx2)
		if err != nil {
			return err
		}

		// get related resources
		resources := resource.resourcesForSlice(ctx2.slice)

		// prepare links
		links := &jsonapi.DocumentLinks{
			Self: base,
		}

		// write result
		return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
	}

	// finish has many relationship
	if relationField.HasMany {
		// prepare filter
		var filterName string

		// find related relationship
		for _, field := range resource.Model.Meta().Fields {
			// TODO: Is this handled correctly?

			if field.RelType == r.Model.Meta().PluralName {
				filterName = field.BSONName
				break
			}
		}

		// check filter name
		if filterName == "" {
			return fmt.Errorf("unable to determine filter name for %s on %s", r.Model.Meta().PluralName, singularName)
		}

		// modify context
		ctx.Request.Intent = jsonapi.ListResources

		// create new request
		ctx2 := resource.buildContext(FindAll, ctx.Request, ctx.GinContext)

		// set query
		ctx2.Query = bson.M{
			filterName: bson.M{
				"$in": []bson.ObjectId{ctx.Model.ID()},
			},
		}

		// load related models
		err := resource.loadModels(ctx2)
		if err != nil {
			return err
		}

		// get related resources
		resources := resource.resourcesForSlice(ctx2.slice)

		// prepare links
		links := &jsonapi.DocumentLinks{
			Self: base,
		}

		// write result
		return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
	}

	return nil
}

func (r *Resource) getRelationship(ctx *Context) error {
	// load model
	err := r.loadModel(ctx)
	if err != nil {
		return err
	}

	// get resource
	resource := r.resourceForModel(ctx.Model)

	// get relationship
	relationship := resource.Relationships[ctx.Request.Relationship]

	// write result
	return jsonapi.WriteResponse(ctx.GinContext.Writer, http.StatusOK, relationship)
}

func (r *Resource) setRelationship(ctx *Context, doc *jsonapi.Document) error {
	// load model
	err := r.loadModel(ctx)
	if err != nil {
		return err
	}

	// assign relationship
	err = r.assignRelationship(ctx, ctx.Request.Relationship, doc)
	if err != nil {
		return err
	}

	// save model
	err = r.saveModel(ctx)
	if err != nil {
		return err
	}

	// write result
	ctx.GinContext.Status(http.StatusNoContent)
	return nil
}

func (r *Resource) appendToRelationship(ctx *Context, doc *jsonapi.Document) error {
	// load model
	err := r.loadModel(ctx)
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
	err = r.saveModel(ctx)
	if err != nil {
		return err
	}

	// write result
	ctx.GinContext.Status(http.StatusNoContent)
	return nil
}

func (r *Resource) removeFromRelationship(ctx *Context, doc *jsonapi.Document) error {
	// load model
	err := r.loadModel(ctx)
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
	err = r.saveModel(ctx)
	if err != nil {
		return err
	}

	// write result
	ctx.GinContext.Status(http.StatusNoContent)
	return nil
}

func (r *Resource) buildContext(action Action, req *jsonapi.Request, gctx *gin.Context) *Context {
	return &Context{
		Action:     action,
		DB:         r.endpoint.db,
		Request:    req,
		GinContext: gctx,
	}
}

func (r *Resource) runCallback(cb Callback, ctx *Context, errorStatus int) error {
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

func (r *Resource) loadModel(ctx *Context) error {
	// validate id
	if !bson.IsObjectIdHex(ctx.Request.ResourceID) {
		return jsonapi.BadRequest("Invalid resource ID")
	}

	// prepare context
	ctx.Query = bson.M{
		"_id": bson.ObjectIdHex(ctx.Request.ResourceID),
	}

	// run authorizer if available
	err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// prepare object
	obj := r.Model.Meta().Make()

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Find(ctx.Query).One(obj)
	if err == mgo.ErrNotFound {
		return jsonapi.NotFound("Resource not found")
	} else if err != nil {
		return err
	}

	// initialize and set model
	ctx.Model = Init(obj.(Model))

	return nil
}

func (r *Resource) loadModels(ctx *Context) error {
	// add filters
	for _, field := range r.Model.Meta().FieldsByTag("filterable") {
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
		for _, field := range r.Model.Meta().FieldsByTag("sortable") {
			if params == field.BSONName || params == "-"+field.BSONName {
				ctx.Sorting = append(ctx.Sorting, params)
			}
		}
	}

	// TODO: Enforce pagination automatically (20 items per page).

	// run authorizer if available
	err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// prepare slice
	ctx.slice = r.Model.Meta().MakeSlice()

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Find(ctx.Query).
		Sort(ctx.Sorting...).All(ctx.slice)
	if err != nil {
		return err
	}

	// initialize slice
	InitSlice(ctx.slice)

	return nil
}

func (r *Resource) assignData(ctx *Context, res *jsonapi.Resource) error {
	// map attributes to struct
	err := jsonapi.MapToStruct(res.Attributes, ctx.Model)
	if err != nil {
		return err
	}

	// iterate relationships
	for name, rel := range res.Relationships {
		err = r.assignRelationship(ctx, name, rel)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Resource) assignRelationship(ctx *Context, name string, rel *jsonapi.Document) error {
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
			if rel.Data.One != nil {
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
				ctx.Model.Set(field.Name, nil)
			}
		}

		// handle to many relationship
		if field.ToMany {
			// prepare slice of ids
			var ids []bson.ObjectId

			// range over all resources
			for _, r := range rel.Data.Many {
				// get id
				id := bson.ObjectIdHex(r.ID)

				// return error for an invalid id
				if !id.Valid() {
					return jsonapi.BadRequest("Invalid relationship ID")
				}

				// add id to slice
				ids = append(ids, id)
			}

			// set ids
			ctx.Model.Set(field.Name, ids)
		}
	}

	return nil
}

func (r *Resource) saveModel(ctx *Context) error {
	// validate model
	err := ctx.Model.Validate(false)
	if err != nil {
		return jsonapi.BadRequest(err.Error())
	}

	// run validator if available
	err = r.runCallback(r.Validator, ctx, http.StatusBadRequest)
	if err != nil {
		return err
	}

	// update model
	return r.endpoint.db.C(r.Model.Meta().Collection).Update(ctx.Query, ctx.Model)
}

func (r *Resource) resourceForModel(model Model) *jsonapi.Resource {
	// prepare resource
	resource := &jsonapi.Resource{
		Type:          r.Model.Meta().PluralName,
		ID:            model.ID().Hex(),
		Attributes:    model,
		Relationships: make(map[string]*jsonapi.Document),
	}

	// generate base link
	base := r.endpoint.prefix + "/" + r.Model.Meta().PluralName + "/" + model.ID().Hex()

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
						ID:   model.Get(field.Name).(bson.ObjectId).Hex(),
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
			// prepare slice of references
			references := make([]*jsonapi.Resource, 0)

			// add all references
			for _, id := range model.Get(field.Name).([]bson.ObjectId) {
				references = append(references, &jsonapi.Resource{
					Type: field.RelType,
					ID:   id.Hex(),
				})
			}

			// assign relationship
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Data: &jsonapi.HybridResource{
					Many: references,
				},
				Links: links,
			}
		} else if field.HasMany {
			// TODO: Load has many references?

			// only set links
			resource.Relationships[field.RelName] = &jsonapi.Document{
				Links: links,
			}
		}
	}

	return resource
}

func (r *Resource) resourcesForSlice(ptr interface{}) []*jsonapi.Resource {
	// dereference pointer to slice
	slice := reflect.ValueOf(ptr).Elem()

	// prepare resources
	resources := make([]*jsonapi.Resource, 0, slice.Len())

	// create resource
	for i := 0; i < slice.Len(); i++ {
		resources = append(resources, r.resourceForModel(slice.Index(i).Interface().(Model)))
	}

	return resources
}
