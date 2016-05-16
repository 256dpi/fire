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

type Action int

const (
	FindAll Action = iota
	FindOne
	Create
	Update
	Delete
)

type Context struct {
	Action     Action
	Model      Model
	ID         bson.ObjectId
	GinContext *gin.Context
	Api2GoReq  *api2go.Request
}

type Callback func(*Context) (error, error)

type Resource struct {
	Model      Model
	Collection string

	Authorizer      Callback
	CreateValidator Callback
	UpdateValidator Callback
	DeleteValidator Callback
	Cleaner         Callback

	adapter  *adapter
	endpoint *Endpoint
}

func (r *Resource) InitializeObject(obj interface{}) {
	// initialize model
	Init(obj.(Model))
}

func (r *Resource) FindAll(req api2go.Request) (api2go.Responder, error) {
	// clean query params
	r.cleanQueryParams(&req)

	// build context
	ctx := r.buildContext(FindAll, &req)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx); err != nil {
		return nil, *err
	}

	// prepare query
	query := bson.M{}

	// add self referencing filter
	if value, ok := getQueryParam(&req, r.Model.getBase().singularName+"-id"); ok {
		query["_id"] = value
	}

	// add to one relationship filters
	for _, rel := range r.Model.getBase().toOneRelationships {
		if value, ok := getQueryParam(&req, rel.name+"-id"); ok {
			query[rel.dbField] = value
		}
	}

	// add filters
	for _, attr := range r.Model.getBase().attributes {
		if value, ok := getQueryParam(&req, "filter["+attr.name+"]"); ok {
			query[attr.dbField] = value
		}
	}

	// prepare slice
	pointer := newSlicePointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Collection).Find(query).All(pointer)
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

func (r *Resource) FindOne(id string, req api2go.Request) (api2go.Responder, error) {
	// validate id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "invalid id", http.StatusBadRequest)
	}

	// build context
	ctx := r.buildContext(FindOne, &req)
	ctx.ID = bson.ObjectIdHex(id)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx); err != nil {
		return nil, *err
	}

	// prepare object
	obj := newStructPointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Collection).FindId(ctx.ID).One(obj)
	if err == mgo.ErrNotFound {
		return nil, api2go.NewHTTPError(err, "resource not found", http.StatusNotFound)
	} else if err != nil {
		return nil, api2go.NewHTTPError(err, "error while retrieving resource", http.StatusInternalServerError)
	}

	// initialize model
	model := Init(obj.(Model))

	return &response{Data: model}, nil
}

func (r *Resource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(Create, &req)
	ctx.Model = obj.(Model)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx); err != nil {
		return nil, *err
	}

	// validate model
	err := ctx.Model.Validate(true)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run delete validator if available
	if err := r.runCallback(r.CreateValidator, ctx); err != nil {
		return nil, *err
	}

	// query db
	err = r.endpoint.db.C(r.Collection).Insert(ctx.Model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while saving resource", http.StatusInternalServerError)
	}

	return &response{Data: ctx.Model, Code: http.StatusCreated}, nil
}

func (r *Resource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(Update, &req)
	ctx.Model = obj.(Model)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx); err != nil {
		return nil, *err
	}

	// validate model
	err := ctx.Model.Validate(false)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run update validator if available
	if err := r.runCallback(r.UpdateValidator, ctx); err != nil {
		return nil, *err
	}

	// query db
	err = r.endpoint.db.C(r.Collection).UpdateId(ctx.Model.getBase().ID, ctx.Model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while updating resource", http.StatusInternalServerError)
	}

	return &response{Data: ctx.Model}, nil
}

func (r *Resource) Delete(id string, req api2go.Request) (api2go.Responder, error) {
	// validate id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "invalid id", http.StatusBadRequest)
	}

	// build context
	ctx := r.buildContext(Delete, &req)
	ctx.ID = bson.ObjectIdHex(id)

	// run authorizer if available
	if err := r.runCallback(r.Authorizer, ctx); err != nil {
		return nil, *err
	}

	// run delete validator if available
	if err := r.runCallback(r.DeleteValidator, ctx); err != nil {
		return nil, *err
	}

	// run cleaner if available
	if err := r.runCallback(r.Cleaner, ctx); err != nil {
		return nil, *err
	}

	// query db
	err := r.endpoint.db.C(r.Collection).RemoveId(ctx.ID)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while deleting resource", http.StatusInternalServerError)
	}

	return &response{Code: http.StatusNoContent}, nil
}

func (r *Resource) buildContext(act Action, req *api2go.Request) *Context {
	return &Context{
		Action:     act,
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

func (r *Resource) runCallback(cb Callback, ctx *Context) *api2go.HTTPError {
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
			httpErr := api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
			return &httpErr
		}
	}

	return nil
}
