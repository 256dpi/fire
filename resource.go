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
	Api2GoReq *api2go.Request
}

// A Resource provides an interface to a model.
type Resource struct {
	// The model that this resource should provide (e.g. &Foo{}).
	Model Model

	// The MongoDB collection that should be used for storage.
	Collection string

	// The Authorizer is run on all actions. Will return a Forbidden status if
	// a user error is returned.
	Authorizer Callback

	// The Validator is run to validate Create, Update and Delete actions. Will
	// return a Bad Request status if a user error is returned.
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
	// clean query params
	r.cleanQueryParams(&req)

	// build context
	ctx := r.buildContext(FindAll, &req)
	ctx.Query = bson.M{}

	// add self referencing filter
	if value, ok := getQueryParam(&req, r.Model.getBase().singularName+"-id"); ok {
		ctx.Query["_id"] = value
	}

	// add to one relationship filters
	for _, rel := range r.Model.getBase().toOneRelationships {
		if value, ok := getQueryParam(&req, rel.name+"-id"); ok {
			ctx.Query[rel.dbField] = value
		}
	}

	// add filters
	for _, attr := range r.Model.getBase().attributes {
		if attr.filterable {
			if value, ok := getQueryParam(&req, "filter["+attr.name+"]"); ok {
				ctx.Query[attr.dbField] = value
			}
		}
	}

	// add sorting
	if fields, ok := req.QueryParams["sort"]; ok {
		for _, field := range fields {
			for _, attr := range r.Model.getBase().attributes {
				if attr.sortable && (field == attr.dbField || field == "-"+attr.dbField) {
					ctx.Sorting = append(ctx.Sorting, field)
				}
			}
		}
	}

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx, http.StatusForbidden); err != nil {
		return nil, *err
	}

	// prepare slice
	pointer := newSlicePointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Collection).Find(ctx.Query).Sort(ctx.Sorting...).All(pointer)
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
	if err := r.runCallback(r.Authorizer, ctx, http.StatusForbidden); err != nil {
		return nil, *err
	}

	// prepare object
	obj := newStructPointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Collection).Find(ctx.Query).One(obj)
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
	if err := r.runCallback(r.Authorizer, ctx, http.StatusForbidden); err != nil {
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
	err = r.endpoint.db.C(r.Collection).Insert(ctx.Model)
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
	if err := r.runCallback(r.Authorizer, ctx, http.StatusForbidden); err != nil {
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
	err = r.endpoint.db.C(r.Collection).Update(ctx.Query, ctx.Model)
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
	if err := r.runCallback(r.Authorizer, ctx, http.StatusForbidden); err != nil {
		return nil, *err
	}

	// run validator if available
	if err := r.runCallback(r.Validator, ctx, http.StatusBadRequest); err != nil {
		return nil, *err
	}

	// query db
	err := r.endpoint.db.C(r.Collection).Remove(ctx.Query)
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
		Api2GoReq:  req,
	}
}

func (r *Resource) cleanQueryParams(req *api2go.Request) {
	for param, values := range req.QueryParams {
		// remove *Name params as not needed
		if strings.HasSuffix(param, "Name") {
			delete(req.QueryParams, param)
		}

		// map *ID to dashed singular names using the endpoints nameMap
		if strings.HasSuffix(param, "ID") {
			pluralName := strings.Replace(param, "ID", "", 1)
			singularName, ok := r.endpoint.nameMap[pluralName]
			if ok {
				delete(req.QueryParams, param)
				req.QueryParams[singularName+"-id"] = values
			}
		}
	}
}

func (r *Resource) runCallback(cb Callback, ctx *Context, errorStatus int) *api2go.HTTPError {
	// check if callback is available
	if cb != nil {
		err, sysErr := cb(ctx)
		if sysErr != nil {
			// return system error
			httpErr := api2go.NewHTTPError(sysErr, "internal server error", http.StatusInternalServerError)
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
