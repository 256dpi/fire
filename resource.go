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
	Action Action
	Model Model
	ID bson.ObjectId
	GinContext *gin.Context
	Api2GoReq *api2go.Request
}

type Authorizer func(*Context) (bool, error)

type Validator func(*Context) (bool, error)

type Cleaner func(*Context) error

type Filter struct {
	Param string
	Field string
}

type Resource struct {
	Model      Model
	Collection string

	// TODO: support query filters using fire:"filter" struct tags.
	QueryFilters []Filter

	Authorizer      Authorizer
	CreateValidator Validator
	UpdateValidator Validator
	DeleteValidator Validator
	Cleaner         Cleaner

	adapter  *adapter
	endpoint *Endpoint
}

func (r *Resource) InitializeObject(obj interface{}) {
	// initialize model
	Init(obj.(Model))
}

func (r *Resource) FindAll(req api2go.Request) (api2go.Responder, error) {
	// build context
	ctx := r.buildContext(FindAll, &req)

	// run authorizer if available
	if r.Authorizer != nil {
		ok, err := r.Authorizer(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// prepare slice
	pointer := newSlicePointer(r.Model)

	// prepare query
	query := bson.M{}

	// process ugly QueryParams
	for param, values := range req.QueryParams {
		// TODO: Use *Name param to lookup relationship type and add the proper
		// filter as the query parameter

		// remove *Name params as not needed
		if strings.HasSuffix(param, "Name") {
			delete(req.QueryParams, param)
		}

		// map *ID to singular names using the resourceNameMap
		if strings.HasSuffix(param, "ID") {
			pluralName := strings.Replace(param, "ID", "", 1)
			singularName, ok := r.endpoint.nameMap[pluralName]
			if ok {
				delete(req.QueryParams, param)
				req.QueryParams[singularName+"-id"] = values
			}
		}
	}

	// add query filters
	for _, filter := range r.QueryFilters {
		if value, ok := getQueryParam(&req, filter.Param); ok {
			query[filter.Field] = value
		}
	}

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
	if r.Authorizer != nil {
		ok, err := r.Authorizer(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
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
	if r.Authorizer != nil {
		ok, err := r.Authorizer(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// validate model
	err := ctx.Model.Validate(true)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run delete validator if available
	if r.CreateValidator != nil {
		ok, err := r.CreateValidator(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while validating creation", http.StatusInternalServerError)
		} else if !ok {
			return nil, api2go.NewHTTPError(nil, "resource cannot be created", http.StatusBadRequest)
		}
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
	if r.Authorizer != nil {
		ok, err := r.Authorizer(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// validate model
	err := ctx.Model.Validate(false)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run update validator if available
	if r.UpdateValidator != nil {
		ok, err := r.UpdateValidator(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while validating update", http.StatusInternalServerError)
		} else if !ok {
			return nil, api2go.NewHTTPError(nil, "resource cannot be updated", http.StatusBadRequest)
		}
	}

	// query db
	err = r.endpoint.db.C(r.Collection).UpdateId(ctx.Model.getObjectID(), ctx.Model)
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
	if r.Authorizer != nil {
		ok, err := r.Authorizer(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// run delete validator if available
	if r.DeleteValidator != nil {
		ok, err := r.DeleteValidator(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while validating deletion", http.StatusInternalServerError)
		} else if !ok {
			return nil, api2go.NewHTTPError(nil, "resource cannot be deleted", http.StatusBadRequest)
		}
	}

	// run cleaner if available
	if r.Cleaner != nil {
		err := r.Cleaner(ctx)
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while cleaning up", http.StatusInternalServerError)
		}
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
		Action: act,
		GinContext: r.adapter.getContext(req),
		Api2GoReq: req,
	}
}
