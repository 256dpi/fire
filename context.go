package fire

import (
	"net/http"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/jsonapi"
	"gopkg.in/mgo.v2/bson"
)

// An Operation indicates the purpose of a yield to a callback in the processing
// flow of an API request by a controller. These operations may occur multiple
// times during a single request.
type Operation int

// All the available operations.
const (
	_ Operation = iota

	// The list operation will used to authorize the loading of multiple
	// resources from a collection.
	//
	// Note: This operation is also used to load related resources.
	List

	// The find operation will be used to authorize the loading of a specific
	// resource from a collection.
	//
	// Note: This operation is also used to load a specific related resource.
	Find

	// The create operation will be used to authorize and validate the creation
	// of a new resource in a collection.
	Create

	// The update operation will be used to authorize the loading and validate
	// the updating of a specific resource in a collection.
	//
	// Note: Updates can include attributes, relationships or both.
	Update

	// The delete operation will be used to authorize the loading and validate
	// the deletion of a specific resource in a collection.
	Delete

	// The collection action operation will be used to authorize the execution
	// of a callback for a collection action.
	CollectionAction

	// The resource action operation will be used to authorize the execution
	// of a callback for a resource action.
	ResourceAction
)

// Read will return true when this operations does only read data.
func (o Operation) Read() bool {
	return o == List || o == Find
}

// Write will return true when this operation does write data.
func (o Operation) Write() bool {
	return o == Create || o == Update || o == Delete
}

// Action will return true when this operation is a collection or resource action.
func (o Operation) Action() bool {
	return o == CollectionAction || o == ResourceAction
}

// A Context provides useful contextual information.
type Context struct {
	// The current operation in process (read only).
	Operation Operation

	// The query that will be used during an List, Find, Update, Delete or
	// ResourceAction operation to select a list of models or a specific model.
	//
	// On Find, Update and Delete operations, the "_id" key is preset to the
	// resource id, while on forwarded List operations a relationship filter is
	// preset.
	Selector bson.M

	// The query that will be used during an List, Find, Update, Delete or
	// ResourceAction operation to further filter the selection of a list of
	// models or a specific model.
	//
	// On List operations, attribute filters are preset.
	Filter bson.M

	// The Model that will be saved during Create, updated during Update or
	// deleted during Delete.
	Model coal.Model

	// The sorting that will be used during List.
	Sorting []string

	// The document that will be written to the client during List, Find, Create
	// and partially Update. The JSON API endpoints which modify a resources
	// relationships do only respond with a header as no other information should
	// be changed.
	//
	// Note: The document will be set before notifiers are run.
	Response *jsonapi.Document

	// The store that is used to retrieve and persist the model (read only).
	Store *coal.SubStore

	// The underlying JSON-API request (read only).
	JSONAPIRequest *jsonapi.Request

	// The underlying HTTP request (read only).
	//
	// Note: The path is not updated when a controller forwards a request to
	// another controller.
	HTTPRequest *http.Request

	// The underlying HTTP response writer. The response writer should only be
	// used during collection or resource actions to write a response or to set
	// custom headers.
	ResponseWriter http.ResponseWriter

	// The Controller that is managing the request (read only).
	Controller *Controller

	// The Group that received the request (read only).
	Group *Group

	Tracer *Tracer

	// The logger is invoked if set with debugging information.
	Logger func(string)

	original coal.Model
}

// Query returns the composite query of Selector and Filter.
func (c *Context) Query() bson.M {
	return bson.M{
		"$and": []bson.M{c.Selector, c.Filter},
	}
}

// Original will return the stored version of the model. This method is intended
// to be used to calculate the changed fields during an Update operation. Any
// returned error is already marked as fatal. This function will cache and reuse
// loaded models between multiple callbacks.
//
// Note: The method will panic if being used during any other operation than Update.
func (c *Context) Original() (coal.Model, error) {
	// begin trace
	c.Tracer.Push("fire/Context.Original")

	// check operation
	if c.Operation != Update {
		panic("fire: the original can only be loaded during an update operation")
	}

	// return cached model
	if c.original != nil {
		c.Tracer.Pop()
		return c.original, nil
	}

	// create a new model
	m := c.Model.Meta().Make()

	// read original document
	c.Tracer.Push("mgo/Query.One")
	c.Tracer.Tag("id", c.Model.ID())
	err := c.Store.C(c.Model).FindId(c.Model.ID()).One(m)
	if err != nil {
		return nil, Fatal(err)
	}
	c.Tracer.Pop()

	// cache model
	c.original = coal.Init(m)

	// finish trace
	c.Tracer.Pop()

	return c.original, nil
}

// Log the specified message if a logger is set.
func (c *Context) Log(str string) {
	if c.Logger != nil {
		c.Logger(str)
	}
}
