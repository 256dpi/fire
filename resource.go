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
	GinContext *gin.Context
	Api2GoReq *api2go.Request
}

type Authorizer func(Action, *Context) (bool, error)

type ChangeValidator func(Model, *Context) (bool, error)

type DeleteValidator func(bson.ObjectId, *Context) (bool, error)

type Cleaner func(bson.ObjectId, *Context) error

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
	CreateValidator ChangeValidator
	UpdateValidator ChangeValidator
	DeleteValidator DeleteValidator
	Cleaner         Cleaner

	adapter  *adapter
	endpoint *Endpoint
}

func (r *Resource) InitializeObject(obj interface{}) {
	// initialize model
	Init(obj.(Model))
}

func (r *Resource) FindAll(req api2go.Request) (api2go.Responder, error) {
	// run authorizer if available
	if r.Authorizer != nil {
		ok, err := r.Authorizer(FindAll, r.buildContext(&req))
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
	// run authorizer if available
	if r.Authorizer != nil {
		ok, err := r.Authorizer(FindOne, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	if !bson.IsObjectIdHex(id) {
		return &response{}, api2go.NewHTTPError(nil, "invalid id", http.StatusBadRequest)
	}

	// prepare object
	obj := newStructPointer(r.Model)

	// query db
	err := r.endpoint.db.C(r.Collection).FindId(bson.ObjectIdHex(id)).One(obj)
	if err == mgo.ErrNotFound {
		return nil, api2go.NewHTTPError(err, "resource not found", http.StatusNotFound)
	} else if err != nil {
		return nil, api2go.NewHTTPError(err, "error while retrieving resource", http.StatusInternalServerError)
	}

	// get model
	model := obj.(Model)

	// initialize model
	Init(model)

	return &response{Data: model}, nil
}

func (r *Resource) Create(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// run authorizer if available
	if r.Authorizer != nil {
		ok, err := r.Authorizer(Create, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// get model
	model := obj.(Model)

	// validate model
	err := model.Validate(true)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run delete validator if available
	if r.CreateValidator != nil {
		ok, err := r.CreateValidator(model, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while validating creation", http.StatusInternalServerError)
		} else if !ok {
			return nil, api2go.NewHTTPError(nil, "resource cannot be created", http.StatusBadRequest)
		}
	}

	// query db
	err = r.endpoint.db.C(r.Collection).Insert(obj)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while saving resource", http.StatusInternalServerError)
	}

	return &response{Data: obj, Code: http.StatusCreated}, nil
}

func (r *Resource) Update(obj interface{}, req api2go.Request) (api2go.Responder, error) {
	// run authorizer if available
	if r.Authorizer != nil {
		ok, err := r.Authorizer(Update, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// get model
	model := obj.(Model)

	// validate model
	err := model.Validate(false)
	if err != nil {
		return nil, api2go.NewHTTPError(nil, err.Error(), http.StatusBadRequest)
	}

	// run update validator if available
	if r.UpdateValidator != nil {
		ok, err := r.UpdateValidator(model, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while validating update", http.StatusInternalServerError)
		} else if !ok {
			return nil, api2go.NewHTTPError(nil, "resource cannot be updated", http.StatusBadRequest)
		}
	}

	// query db
	err = r.endpoint.db.C(r.Collection).UpdateId(bson.ObjectIdHex(model.GetID()), model)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while updating resource", http.StatusInternalServerError)
	}

	return &response{Data: obj}, nil
}

func (r *Resource) Delete(id string, req api2go.Request) (api2go.Responder, error) {
	// run authorizer if available
	if r.Authorizer != nil {
		ok, err := r.Authorizer(Delete, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while authorizing action", http.StatusInternalServerError)
		}
		if !ok {
			return nil, api2go.NewHTTPError(nil, "client not authorized", http.StatusForbidden)
		}
	}

	// validate supplied id
	if !bson.IsObjectIdHex(id) {
		return nil, api2go.NewHTTPError(nil, "invalid id", http.StatusBadRequest)
	}

	// get object id
	oid := bson.ObjectIdHex(id)

	// run delete validator if available
	if r.DeleteValidator != nil {
		ok, err := r.DeleteValidator(oid, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while validating deletion", http.StatusInternalServerError)
		} else if !ok {
			return nil, api2go.NewHTTPError(nil, "resource cannot be deleted", http.StatusBadRequest)
		}
	}

	// run cleaner if available
	if r.Cleaner != nil {
		err := r.Cleaner(oid, r.buildContext(&req))
		if err != nil {
			return nil, api2go.NewHTTPError(err, "error while cleaning up", http.StatusInternalServerError)
		}
	}

	// query db
	err := r.endpoint.db.C(r.Collection).RemoveId(oid)
	if err != nil {
		return nil, api2go.NewHTTPError(err, "error while deleting resource", http.StatusInternalServerError)
	}

	return &response{Code: http.StatusNoContent}, nil
}

func (r *Resource) buildContext(req *api2go.Request) *Context {
	return &Context{
		GinContext: r.adapter.getContext(req),
		Api2GoReq: req,
	}
}
