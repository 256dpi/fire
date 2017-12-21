package fire

import (
	"net/http"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// An Operation is a single yield to a callback in the processing flow of an
// API request by a controller. These operations may occur multiple times
// during a single request.
type Operation int

// All the available operations.
const (
	_ Operation = iota
	List
	Find
	Create
	Update
	Delete
	Custom
)

// Read will return true when this operations does only read data.
func (o Operation) Read() bool {
	return o == List || o == Find
}

// Write will return true when this operation does write data.
func (o Operation) Write() bool {
	return o == Create || o == Update || o == Delete
}

// A Context provides useful contextual information.
type Context struct {
	// The current operation in process.
	Operation Operation

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

	// TODO: Rename to action?

	// The custom action object is set when a Custom action is processed.
	CustomAction *CustomAction

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

	original coal.Model
}

// Original will return the stored version of the model. This method is intended
// to be used to calculate the changed fields during an Update operation. Any
// returned error is already marked as fatal. This function will cache and reuse
// loaded models between multiple callbacks.
//
// Note: The method will panic if being used during any other operation than Update.
func (c *Context) Original() (coal.Model, error) {
	if c.Operation != Update {
		panic("fire: the original can only be loaded during an update operation")
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

// CustomAction contains information to process a custom action.
type CustomAction struct {
	// The name of the action.
	Name string

	// What type of actions that is being processed.
	CollectionAction bool
	ResourceAction   bool

	// The resource id for a resource action.
	ResourceID string

	// The response that will be written to the client while processing a custom
	// collection or resource action. If set, the value must be either a byte
	// slice for raw responses or a json.Marshal compatible object for json
	// responses.
	Response interface{}

	// CustomContentType denotes the content type of the custom action response.
	//
	// Note: This value is only considered if the response is set to a byte slice.
	ContentType string

	// TODO: Encode as a JSON-API response if the custom response is an instance
	// of the controller model.
}
