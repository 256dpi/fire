package fire

import (
	"net/http"

	"github.com/gonfire/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// An Action describes the currently called action on the API.
type Action int

// All the available actions.
const (
	_ Action = iota
	List
	Find
	Create
	Update
	Delete
)

// Read will return true when this action does only read data.
func (a Action) Read() bool {
	return a == List || a == Find
}

// Write will return true when this action does write data.
func (a Action) Write() bool {
	return a == Create || a == Update || a == Delete
}

// A Context provides useful contextual information.
type Context struct {
	// The current action in process.
	Action Action

	// The query that will be used during FindAll, FindOne, Update or Delete.
	// On FindOne, Update and Delete, the "_id" key is preset to the document ID.
	// On FindAll all field filters and relationship filters are preset.
	Query bson.M

	// The Model that will be saved during Create or Update.
	Model Model

	// The sorting that will be used during FindAll.
	Sorting []string

	// The store that is used to retrieve and persist the model.
	Store *Store

	// The underlying JSON API request.
	JSONAPIRequest *jsonapi.Request

	// The underlying HTTP request.
	HTTPRequest *http.Request

	prefix string
	group  *Group

	original Model
}

func buildContext(prefix string, group *Group, store *Store, req *jsonapi.Request, r *http.Request) *Context {
	return &Context{
		prefix:         prefix,
		group:          group,
		Store:          store,
		JSONAPIRequest: req,
		HTTPRequest:    r,
	}
}

// Original will return the stored version of the model. This method is intended
// to be used to calculate the changed fields during an Update action.
//
// Note: The method will directly return any mgo errors and panic if being used
// during any other action than Update.
func (c *Context) Original() (Model, error) {
	if c.Action != Update {
		panic("Original can only be used during an Update action")
	}

	// return cached model
	if c.original != nil {
		return c.original, nil
	}

	// create a new model
	m := c.Model.Meta().Make()

	// read original document
	err := c.Store.C(c.Model).FindId(c.Model.ID()).One(m)
	if err != nil {
		// TODO: Report error.
		return nil, err
	}

	// cache model
	c.original = Init(m)

	return c.original, nil
}
