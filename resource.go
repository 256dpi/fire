package fire

import (
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
		r.listResources(ctx)
	case jsonapi.FindResource:
		ctx := r.buildContext(FindOne, req, gctx)
		r.findResource(ctx)
	case jsonapi.CreateResource:
		ctx := r.buildContext(Create, req, gctx)
		r.createResource(ctx, doc)
	case jsonapi.UpdateResource:
		ctx := r.buildContext(Update, req, gctx)
		r.updateResource(ctx, doc)
	case jsonapi.DeleteResource:
		ctx := r.buildContext(Delete, req, gctx)
		r.deleteResource(ctx)
	case jsonapi.GetRelatedResources:
		// TODO: Could also be a find one...
		ctx := r.buildContext(FindAll, req, gctx)
		r.getRelatedResources(ctx)
	case jsonapi.GetRelationship:
		// TODO: Could also be a find one...
		ctx := r.buildContext(FindAll, req, gctx)
		r.getRelationship(ctx)
	case jsonapi.SetRelationship:
		ctx := r.buildContext(Update, req, gctx)
		r.setRelationship(ctx, doc)
	case jsonapi.AppendToRelationship:
		ctx := r.buildContext(Update, req, gctx)
		r.appendToRelationship(ctx, doc)
	case jsonapi.RemoveFromRelationship:
		ctx := r.buildContext(Update, req, gctx)
		r.removeFromRelationship(ctx, doc)
	}

	// write any occurring errors
	if err != nil {
		jsonapi.WriteError(gctx.Writer, err)
		gctx.Error(err)
		gctx.Abort()
	}
}

func (r *Resource) listResources(ctx *Context) error {
	// prepare context
	ctx.Query = bson.M{}

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

	// run authorizer if available
	err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized)
	if err != nil {
		return err
	}

	// prepare slice
	pointer := r.Model.Meta().MakeSlice()

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Find(ctx.Query).
		Sort(ctx.Sorting...).All(pointer)
	if err != nil {
		return err
	}

	// initialize slice
	InitSlice(pointer)

	// get slice
	slice := reflect.ValueOf(pointer).Elem()

	// prepare resources
	resources := make([]*jsonapi.Resource, 0, slice.Len())

	// create resource
	for i := 0; i < slice.Len(); i++ {
		resources = append(resources, r.resourceForModel(slice.Index(i).Interface().(Model)))
	}

	// prepare links
	links := &jsonapi.DocumentLinks{
		Self: r.endpoint.prefix + "/" + r.Model.Meta().PluralName,
	}

	// TODO: Enforce pagination automatically (20 items per page).

	// write result
	return jsonapi.WriteResources(ctx.GinContext.Writer, http.StatusOK, resources, links)
}

func (r *Resource) findResource(ctx *Context) error {
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
	if doc.Data.One == nil {
		return jsonapi.BadRequest("Resource object expected")
	}

	// create new model
	ctx.Model = Init(r.Model.Meta().Make())

	// assign attributes
	err := r.assignAttributesAndRelationships(ctx, doc.Data.One)
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
	if doc.Data.One == nil {
		return jsonapi.BadRequest("Resource object expected")
	}

	err := r.loadModel(ctx)
	if err != nil {
		return err
	}

	// assign attributes
	err = r.assignAttributesAndRelationships(ctx, doc.Data.One)
	if err != nil {
		return err
	}

	// validate model
	err = ctx.Model.Validate(false)
	if err != nil {
		return jsonapi.BadRequest(err.Error())
	}

	// run validator if available
	err = r.runCallback(r.Validator, ctx, http.StatusBadRequest)
	if err != nil {
		return err
	}

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Update(ctx.Query, ctx.Model)
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
	return nil
}

func (r *Resource) getRelationship(ctx *Context) error {
	return nil
}

func (r *Resource) setRelationship(ctx *Context, doc *jsonapi.Document) error {
	return nil
}

func (r *Resource) appendToRelationship(ctx *Context, doc *jsonapi.Document) error {
	return nil
}

func (r *Resource) removeFromRelationship(ctx *Context, doc *jsonapi.Document) error {
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

//func (r *Resource) setRelationshipFilters(ctx *Context) error {
//	// TODO: This is very cumbersome, let's fix it upstream.
//
//	for param, values := range ctx.API2GoReq.QueryParams {
//		// handle *ID params
//		if strings.HasSuffix(param, "ID") {
//			// get plural name
//			pluralName := strings.Replace(param, "ID", "", 1)
//
//			// ret relation name
//			relName := ctx.API2GoReq.QueryParams[pluralName+"Name"][0]
//
//			// remove params in any case
//			delete(ctx.API2GoReq.QueryParams, param)
//			delete(ctx.API2GoReq.QueryParams, pluralName+"Name")
//
//			// get singular name and continue if not existing
//			singularName, ok := r.endpoint.nameMap[pluralName]
//			if !ok {
//				continue
//			}
//
//			// check if self referencing
//			if singularName == r.Model.Meta().SingularName {
//				ctx.Query["_id"] = bson.M{"$in": stringsToIDs(values)}
//			}
//
//			for _, field := range r.Model.Meta().Fields {
//				// add to one relationship filter
//				if field.ToOne && field.RelName == singularName {
//					ctx.Query[field.BSONName] = bson.M{"$in": stringsToIDs(values)}
//				}
//
//				// add to many relationship filter
//				if field.ToMany && field.RelName == pluralName {
//					ctx.Query[field.BSONName] = bson.M{"$in": stringsToIDs(values)}
//				}
//
//				// add has many relationship filter
//				if field.HasMany && field.RelName == pluralName {
//					// get referenced resource and continue if not existing
//					resource, ok := r.endpoint.resourceMap[singularName]
//					if !ok {
//						continue
//					}
//
//					// prepare key field
//					var keyField string
//
//					// get foreign field
//					for _, field := range resource.Model.Meta().Fields {
//						if field.RelName == relName {
//							keyField = field.BSONName
//						}
//					}
//
//					// check key field
//					if keyField == "" {
//						return api2go.NewHTTPError(nil, "Error while retrieving key field", http.StatusInternalServerError)
//					}
//
//					// read the referenced ids
//					var ids []bson.ObjectId
//					err := ctx.DB.C(resource.Model.Meta().Collection).Find(bson.M{
//						"_id": bson.M{"$in": stringsToIDs(values)},
//					}).Distinct(keyField, &ids)
//					if err != nil {
//						return api2go.NewHTTPError(err, "Error while retrieving resources", http.StatusInternalServerError)
//					}
//
//					ctx.Query["_id"] = bson.M{"$in": ids}
//				}
//			}
//		}
//	}
//
//	return nil
//}

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

func (r *Resource) assignAttributesAndRelationships(ctx *Context, res *jsonapi.Resource) error {
	// map attributes to struct
	err := jsonapi.MapToStruct(res.Attributes, ctx.Model)
	if err != nil {
		return err
	}

	// assign relationships
	for _, field := range ctx.Model.Meta().Fields {
		// check if field is a relationship
		if !field.ToOne && !field.ToMany {
			continue
		}

		// get relationship data and continue if missing
		rel, ok := res.Relationships[field.RelName]
		if !ok {
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
				id := bson.ObjectId(r.ID)

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
				if oid == nil {
					continue
				}

				// create reference
				reference = &jsonapi.Resource{
					Type: field.RelType,
					ID:   model.Get(field.Name).(bson.ObjectId).Hex(),
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
			var references []*jsonapi.Resource

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
