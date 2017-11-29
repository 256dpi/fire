package fire

import (
	"net/http"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
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
	Custom
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

	// The query that will be used during List, Find, Update or Delete to fetch
	// a list of models or the specific requested model.
	//
	// On Find, Update and Delete, the "_id" key is preset to the document ID
	// while on List all field and relationship filters are preset.
	Query bson.M

	// The Model that will be saved during Create, updated during Update or
	// deleted during Delete.
	Model coal.Model

	// The sorting that will be used during List.
	Sorting []string

	// The document that will be written to the client during List, Find, Create
	// and partially Update. The JSON API endpoints to modify a resources
	// relationships do only respond with a header as no other information should
	// be changed.
	//
	// Note: The document will be set before notifiers are run.
	Response *jsonapi.Document

	// The store that is used to retrieve and persist the model.
	Store *coal.SubStore

	// The underlying JSON API request.
	JSONAPIRequest *jsonapi.Request

	// The underlying HTTP request.
	HTTPRequest *http.Request

	// The Controller that is managing the request.
	Controller *Controller

	// The Group that received the request.
	Group *Group

	// The payload that was received with the collection or resource action.
	ActionPayload []byte

	// The response that will be written to the client while processing a
	// collection or resource action. If set, the value must be a json.Marshal
	// compatible object.
	ActionResponse interface{}

	original coal.Model
}

// Original will return the stored version of the model. This method is intended
// to be used to calculate the changed fields during an Update action. Any
// returned error is already marked as fatal.
//
// Note: The method will panic if being used during any other action than Update.
func (c *Context) Original() (coal.Model, error) {
	if c.Action != Update {
		panic("fire: the original can only be loaded during an update action")
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
		return nil, Fatal(err)
	}

	// cache model
	c.original = coal.Init(m)

	return c.original, nil
}
