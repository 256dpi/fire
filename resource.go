package fire

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/manyminds/api2go"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// An Action describes the currently called action on the API.
type Action int

// All the available actions.
const (
	FindAll Action = iota
	FindOne
	Create
	Update
	Delete
)

// A Context provides useful contextual information to callbacks.
type Context struct {
	// The action that has been called.
	Action Action

	// The Model that will be saved during Create and Update.
	Model Model

	// The query that will be used during FindAll, FindOne, Update and Delete.
	//
	// Note: On FindOne, Update and Delete, the "_id" key is already set to the
	// document ID. On FindAll all ordinary filters and relationship filters are
	// also already present.
	Query bson.M

	// The sorting that will be used during FindAll.
	//
	// Note: This list will already include allowed sorting parameters passed
	// in the query parameters.
	Sorting []string

	// The db used to query.
	DB *mgo.Database

	// The underlying gin.context.
	GinContext *gin.Context

	// The underlying api2go.Request
	API2GoReq *api2go.Request
}

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

	// set self referencing and to one relationship filters
	r.setRelationshipFilters(ctx)

	// add filters
	for _, attr := range r.Model.getBase().attributesByTag("filterable") {
		if values, ok := ctx.API2GoReq.QueryParams["filter["+attr.jsonName+"]"]; ok {
			ctx.Query[attr.bsonName] = bson.M{"$in": values}
		}
	}

	// add sorting
	if fields, ok := req.QueryParams["sort"]; ok {
		for _, field := range fields {
			for _, attr := range r.Model.getBase().attributesByTag("sortable") {
				if field == attr.bsonName || field == "-"+attr.bsonName {
					ctx.Sorting = append(ctx.Sorting, field)
				}
			}
		}
	}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, *err
	}

	// prepare slice
	pointer := newSlicePointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Model.Collection()).Find(ctx.Query).Sort(ctx.Sorting...).All(pointer)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while retrieving resources", http.StatusInternalServerError)
	}

	// get content from pointer
	slice := sliceContent(pointer)

	// initialize each model
	s := reflect.ValueOf(slice)
	for i := 0; i < s.Len(); i++ {
		Init(s.Index(i).Interface().(Model))
	}

	return &response{Data: slice}, nil
}

// FindOne implements a part of the api2go.CRUD interface.
func (r *Resource) FindOne(id string, req api2go.Request) (api2go.Responder, error) {
	// validate id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "invalid id", http.StatusBadRequest)
	}

	// build context
	ctx := r.buildContext(FindOne, &req)
	ctx.Query = bson.M{"_id": bson.ObjectIdHex(id)}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, *err
	}

	// prepare object
	obj := newStructPointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Model.Collection()).Find(ctx.Query).One(obj)
	if err == mgo.ErrNotFound {
		return nil, api2go.NewHTTPError(err, "resource not found", http.StatusNotFound)
	} else if err != nil {
		return nil, api2go.NewHTTPError(err, "error while retrieving resource", http.StatusInternalServerError)
	}

	// initialize model
	model := Init(obj.(Model))

	return &response{Data: model}, nil
}

// Create implements a part of the api2go.CRUD interface.
func (r *Resource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(Create, &req)
	ctx.Model = obj.(Model)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, *err
	}

	// validate model
	err := ctx.Model.Validate(true)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, *err
	}

	// query db
	err = r.endpoint.db.C(r.Model.Collection()).Insert(ctx.Model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while saving resource", http.StatusInternalServerError)
	}

	return &response{Data: ctx.Model, Code: http.StatusCreated}, nil
}

// Update implements a part of the api2go.CRUD interface.
func (r *Resource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(Update, &req)
	ctx.Model = obj.(Model)
	ctx.Query = bson.M{"_id": ctx.Model.ID()}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, *err
	}

	// validate model
	err := ctx.Model.Validate(false)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, *err
	}

	// query db
	err = r.endpoint.db.C(r.Model.Collection()).Update(ctx.Query, ctx.Model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while updating resource", http.StatusInternalServerError)
	}

	return &response{Data: ctx.Model}, nil
}

// Delete implements a part of the api2go.CRUD interface.
func (r *Resource) Delete(id string, req api2go.Request) (api2go.Responder, error) {
	// validate id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "invalid id", http.StatusBadRequest)
	}

	// build context
	ctx := r.buildContext(Delete, &req)
	ctx.Query = bson.M{"_id": bson.ObjectIdHex(id)}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusUnauthorized); err != nil {
		return nil, *err
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, *err
	}

	// query db
	err := r.endpoint.db.C(r.Model.Collection()).Remove(ctx.Query)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while deleting resource", http.StatusInternalServerError)
	}

	return &response{Code: http.StatusNoContent}, nil
}

func (r *Resource) buildContext(act Action, req *api2go.Request) *Context {
	return &Context{
		Action:     act,
		DB:         r.endpoint.db,
		GinContext: r.adapter.getContext(req),
		API2GoReq:  req,
	}
}

func (r *Resource) setRelationshipFilters(ctx *Context) {
	for param, values := range ctx.API2GoReq.QueryParams {
		// remove *Name params as not needed
		if strings.HasSuffix(param, "Name") {
			delete(ctx.API2GoReq.QueryParams, param)
		}

		// handle *ID params
		if strings.HasSuffix(param, "ID") {
			// remove param in any case
			delete(ctx.API2GoReq.QueryParams, param)

			// get plural name
			pluralName := strings.Replace(param, "ID", "", 1)

			// get singular name and continue if not existing
			singularName, ok := r.endpoint.nameMap[pluralName]
			if !ok {
				continue
			}

			// check if self referencing
			if singularName == r.Model.getBase().singularName {
				ctx.Query["_id"] = bson.M{"$in": stringsToIDs(values)}
			}

			// add to one relationship filters
			for _, rel := range r.Model.getBase().toOneRelationships {
				if rel.name == singularName {
					ctx.Query[rel.bsonName] = bson.M{"$in": stringsToIDs(values)}
				}
			}
		}
	}
}

func (r *Resource) runCallback(cb Callback, ctx *Context, errorStatus int) *api2go.HTTPError {
	// check if callback is available
	if cb != nil {
		err := cb(ctx)
		if isFatal(err) {
			// return system error
			httpErr := api2go.NewHTTPError(err, "internal server error", http.StatusInternalServerError)
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
