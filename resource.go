package fire

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/manyminds/api2go"
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

	adapter  *adapter
	endpoint *Endpoint
}

/* api2go interface */

// InitializeObject implements the api2go.ObjectInitializer interface.
func (r *Resource) InitializeObject(obj interface{}) {
	// initialize model
	Init(obj.(Model))
}

// FindAll implements the api2go.FindAll interface.
func (r *Resource) FindAll(req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(FindAll, &req)
	ctx.Query = bson.M{}

	// set relationship filters
	err := r.setRelationshipFilters(ctx)
	if err != nil {
		return nil, err
	}

	// add filters
	for _, field := range r.Model.Meta().FieldsByTag("filterable") {
		if values, ok := req.QueryParams["filter["+field.JSONName+"]"]; ok {
			if field.Type == reflect.Bool && len(values) == 1 {
				ctx.Query[field.BSONName] = values[0] == "true"
				continue
			}

			ctx.Query[field.BSONName] = bson.M{"$in": values}
		}
	}

	// add sorting
	if sortParam, ok := req.QueryParams["sort"]; ok {
		for _, params := range sortParam {
			for _, field := range r.Model.Meta().FieldsByTag("sortable") {
				if params == field.BSONName || params == "-"+field.BSONName {
					ctx.Sorting = append(ctx.Sorting, params)
				}
			}
		}
	}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, err
	}

	// prepare slice
	pointer := r.Model.Meta().MakeSlice()

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Find(ctx.Query).Sort(ctx.Sorting...).All(pointer)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "Error while retrieving resources", http.StatusInternalServerError)
	}

	// initialize slice
	InitSlice(pointer)

	// api2go needs a direct slice reference
	slice := reflect.ValueOf(pointer).Elem().Interface()

	return &api2go.Response{Res: slice, Code: http.StatusOK}, nil
}

// FindOne implements a part of the api2go.CRUD interface.
func (r *Resource) FindOne(id string, req api2go.Request) (api2go.Responder, error) {
	// validate id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "Invalid id", http.StatusBadRequest)
	}

	// build context
	ctx := r.buildContext(FindOne, &req)
	ctx.Query = bson.M{"_id": bson.ObjectIdHex(id)}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, err
	}

	// prepare object
	obj := r.Model.Meta().Make()

	// query db
	err := r.endpoint.db.C(r.Model.Meta().Collection).Find(ctx.Query).One(obj)
	if err == mgo.ErrNotFound {
		return nil, api2go.NewHTTPError(err, "Resource not found", http.StatusNotFound)
	} else if err != nil {
		return nil, api2go.NewHTTPError(err, "Error while retrieving resource", http.StatusInternalServerError)
	}

	// initialize model
	model := Init(obj.(Model))

	return &api2go.Response{Res: model, Code: http.StatusOK}, nil
}

// Create implements a part of the api2go.CRUD interface.
func (r *Resource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(Create, &req)
	ctx.Model = obj.(Model)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, err
	}

	// validate model
	err := ctx.Model.Validate(true)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, err
	}

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Insert(ctx.Model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "Error while saving resource", http.StatusInternalServerError)
	}

	return &api2go.Response{Res: ctx.Model, Code: http.StatusCreated}, nil
}

// Update implements a part of the api2go.CRUD interface.
func (r *Resource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(Update, &req)
	ctx.Model = obj.(Model)
	ctx.Query = bson.M{"_id": ctx.Model.ID()}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, err
	}

	// validate model
	err := ctx.Model.Validate(false)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, err
	}

	// query db
	err = r.endpoint.db.C(r.Model.Meta().Collection).Update(ctx.Query, ctx.Model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "Error while updating resource", http.StatusInternalServerError)
	}

	return &api2go.Response{Res: ctx.Model, Code: http.StatusOK}, nil
}

// Delete implements a part of the api2go.CRUD interface.
func (r *Resource) Delete(id string, req api2go.Request) (api2go.Responder, error) {
	// validate id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "Invalid id", http.StatusBadRequest)
	}

	// build context
	ctx := r.buildContext(Delete, &req)
	ctx.Query = bson.M{"_id": bson.ObjectIdHex(id)}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, err
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, err
	}

	// query db
	err := r.endpoint.db.C(r.Model.Meta().Collection).Remove(ctx.Query)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "Error while deleting resource", http.StatusInternalServerError)
	}

	return &api2go.Response{Code: http.StatusNoContent}, nil
}

func (r *Resource) buildContext(act Action, req *api2go.Request) *Context {
	return &Context{
		Action:     act,
		DB:         r.endpoint.db,
		GinContext: r.adapter.getContext(req),
		API2GoReq:  req,
	}
}

func (r *Resource) setRelationshipFilters(ctx *Context) error {
	// TODO: This is very cumbersome, let's fix it upstream.

	for param, values := range ctx.API2GoReq.QueryParams {
		// handle *ID params
		if strings.HasSuffix(param, "ID") {
			// get plural name
			pluralName := strings.Replace(param, "ID", "", 1)

			// ret relation name
			relName := ctx.API2GoReq.QueryParams[pluralName+"Name"][0]

			// remove params in any case
			delete(ctx.API2GoReq.QueryParams, param)
			delete(ctx.API2GoReq.QueryParams, pluralName+"Name")

			// get singular name and continue if not existing
			singularName, ok := r.endpoint.nameMap[pluralName]
			if !ok {
				continue
			}

			// check if self referencing
			if singularName == r.Model.Meta().SingularName {
				ctx.Query["_id"] = bson.M{"$in": stringsToIDs(values)}
			}

			for _, field := range r.Model.Meta().Fields {
				// add to one relationship filter
				if field.ToOne && field.RelName == singularName {
					ctx.Query[field.BSONName] = bson.M{"$in": stringsToIDs(values)}
				}

				// add to many relationship filter
				if field.ToMany && field.RelName == pluralName {
					ctx.Query[field.BSONName] = bson.M{"$in": stringsToIDs(values)}
				}

				// add has many relationship filter
				if field.HasMany && field.RelName == pluralName {
					// get referenced resource and continue if not existing
					resource, ok := r.endpoint.resourceMap[singularName]
					if !ok {
						continue
					}

					// prepare key field
					var keyField string

					// get foreign field
					for _, field := range resource.Model.Meta().Fields {
						if field.RelName == relName {
							keyField = field.BSONName
						}
					}

					// check key field
					if keyField == "" {
						return api2go.NewHTTPError(nil, "Error while retrieving key field", http.StatusInternalServerError)
					}

					// read the referenced ids
					var ids []bson.ObjectId
					err := ctx.DB.C(resource.Model.Meta().Collection).Find(bson.M{
						"_id": bson.M{"$in": stringsToIDs(values)},
					}).Distinct(keyField, &ids)
					if err != nil {
						return api2go.NewHTTPError(err, "Error while retrieving resources", http.StatusInternalServerError)
					}

					ctx.Query["_id"] = bson.M{"$in": ids}
				}
			}
		}
	}

	return nil
}

func (r *Resource) runCallback(cb Callback, ctx *Context, errorStatus int) *api2go.HTTPError {
	// check if callback is available
	if cb != nil {
		err := cb(ctx)
		if isFatal(err) {
			// return system error
			httpErr := api2go.NewHTTPError(err, "Internal server error", http.StatusInternalServerError)
			return &httpErr
		}
		if err != nil {
			// return user error
			httpErr := api2go.NewHTTPError(nil, err.Error(), errorStatus)
			return &httpErr
		}
	}

	return nil
}
