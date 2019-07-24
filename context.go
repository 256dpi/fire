package fire

import (
	"encoding/json"
	"net/http"

	"github.com/256dpi/jsonapi"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/256dpi/fire/coal"
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

// String returns the name of the operation.
func (o Operation) String() string {
	switch o {
	case List:
		return "List"
	case Find:
		return "Find"
	case Create:
		return "Create"
	case Update:
		return "Update"
	case Delete:
		return "Delete"
	case CollectionAction:
		return "CollectionAction"
	case ResourceAction:
		return "ResourceAction"
	}

	return ""
}

// A Context provides useful contextual information.
type Context struct {
	// Data can be used to carry data between callbacks.
	Data Map

	// The current operation in process.
	//
	// Usage: Read Only, Availability: Authorizers
	Operation Operation

	// The query that will be used during an List, Find, Update, Delete or
	// ResourceAction operation to select a list of models or a specific model.
	//
	// On Find, Update and Delete operations, the "_id" key is preset to the
	// resource id, while on forwarded List operations the relationship filter
	// is preset.
	//
	// Usage: Read Only, Availability: Authorizers
	// Operations: !Create, !CollectionAction
	Selector bson.M

	// The filters that will be used during an List, Find, Update, Delete or
	// ResourceAction operation to further filter the selection of a list of
	// models or a specific model.
	//
	// On List operations, attribute and relationship filters are preset.
	//
	// Usage: Append Only, Availability: Authorizers
	// Operations: !Create, !CollectionAction
	Filters []bson.M

	// The sorting that will be used during List.
	//
	// Usage: No Restriction, Availability: Authorizers
	// Operations: List
	Sorting []string

	// Only the whitelisted readable fields are exposed to the client as
	// attributes and relationships.
	//
	// Usage: Reduce Only, Availability: Authorizers
	// Operations: !Delete, !ResourceAction, !CollectionAction
	ReadableFields []string

	// Only the whitelisted writable fields can be altered by requests.
	//
	// Usage: Reduce Only, Availability: Authorizers
	// Operations: Create, Update
	WritableFields []string

	// The Model that will be created, updated, deleted or is requested by a
	// resource action.
	//
	// Usage: Modify Only, Availability: Validators
	// Operations: Create, Update, Delete, ResourceAction
	Model coal.Model

	// The models that will will be returned for a List operation.
	//
	// Usage: Modify Only, Availability: Decorators
	// Operations: List
	Models []coal.Model

	// The original model that is being updated. Can be used to lookup up
	// original values of changed fields.
	//
	// Usage: Ready Only, Availability: Validators
	// Operations: Update
	Original coal.Model

	// The document that will be written to the client.
	//
	// Usage: Modify Only, Availability: Notifiers,
	// Operations: !CollectionAction, !ResourceAction
	Response *jsonapi.Document

	// The store that is used to retrieve and persist the model.
	//
	// Usage: Read Only
	Store *coal.Store

	// The session used by create, update and delete operations if transactions
	// have been enabled.
	//
	// Usage: Read Only
	// Operations: Create, Update, Delete
	Session mongo.SessionContext

	// The underlying JSON-API request.
	//
	// Usage: Read Only
	JSONAPIRequest *jsonapi.Request

	// The underlying HTTP request.
	//
	// Note: The path is not updated when a controller forwards a request to
	// a related controller.
	//
	// Usage: Read Only
	HTTPRequest *http.Request

	// The underlying HTTP response writer. The response writer should only be
	// used during collection or resource actions to write a custom response.
	//
	// Usage: Read Only
	ResponseWriter http.ResponseWriter

	// The Controller that is managing the request.
	//
	// Usage: Read Only
	Controller *Controller

	// The Group that received the request.
	//
	// Usage: Read Only
	Group *Group

	// The Tracer used to trace code execution.
	//
	// Usage: Read Only
	Tracer *Tracer
}

// TC is a shorthand to get a traced collection for the specified model.
func (c *Context) TC(model coal.Model) *coal.TracedCollection {
	return c.Store.TC(c.Tracer, model)
}

// Query returns the composite query of Selector and Filter.
func (c *Context) Query() bson.M {
	// prepare sub queries
	var subQueries []bson.M

	// add selector if present
	if len(c.Selector) > 0 {
		subQueries = append(subQueries, c.Selector)
	}

	// add filters
	subQueries = append(subQueries, c.Filters...)

	// return empty query if no sub queries are present
	if len(subQueries) == 0 {
		return bson.M{}
	}

	// otherwise return $and query
	return bson.M{"$and": subQueries}
}

// Parse will decode a custom JSON body to the specified object.
func (c *Context) Parse(obj interface{}) error {
	// unmarshal json
	err := json.NewDecoder(c.HTTPRequest.Body).Decode(obj)
	if err != nil {
		return err
	}

	return nil
}

// Respond will encode the provided value as JSON and write it to the client.
func (c *Context) Respond(value interface{}) error {
	// encode response
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// write token
	_, err = c.ResponseWriter.Write(bytes)
	if err != nil {
		return err
	}

	return nil
}
