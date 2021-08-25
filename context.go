package fire

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// An Operation indicates the purpose of a yield to a callback in the processing
// flow of an API request by a controller. These operations may occur multiple
// times during a single request.
type Operation int

// All the available operations.
const (
	// List operation will be used to authorize the loading of multiple
	// resources from a collection.
	//
	// Note: This operation is also used to load related resources.
	List Operation = 1 << iota

	// Find operation will be used to authorize the loading of a specific
	// resource from a collection.
	//
	// Note: This operation is also used to load a specific related resource.
	Find

	// Create operation will be used to authorize and validate the creation
	// of a new resource in a collection.
	Create

	// Update operation will be used to authorize the loading and validate
	// the updating of a specific resource in a collection.
	//
	// Note: Updates can include attributes, relationships or both.
	Update

	// Delete operation will be used to authorize the loading and validate
	// the deletion of a specific resource in a collection.
	Delete

	// CollectionAction operation will be used to authorize the execution
	// of a callback for a collection action.
	CollectionAction

	// ResourceAction operation will be used to authorize the execution of a
	// callback for a resource action.
	ResourceAction
)

// Read will return true when the operations only reads data.
func (o Operation) Read() bool {
	return o == List || o == Find
}

// Write will return true when the operation writes data.
func (o Operation) Write() bool {
	return o == Create || o == Update || o == Delete
}

// Action will return true when the operation is a collection or resource action.
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

// Context carries the state of a request and allows callbacks to influence the
// processing of a request.
type Context struct {
	// The context that is cancelled when the timeout has been exceeded or the
	// underlying connection transport has been closed. It may also carry the
	// database session if a transaction is used.
	//
	// Values: lungo.ISessionContext?, trace.Span, *xo.Tracer
	context.Context

	// The custom data map.
	Data stick.Map

	// The current callback stage.
	Stage Stage

	// The current operation in process.
	//
	// Usage: Read Only
	// Availability: Authorizers
	Operation Operation

	// The query that will be used during a List, Find, Update, Delete or
	// ResourceAction operation to select a list of models or a specific model.
	//
	// On Find, Update and Delete operations, the "_id" key is preset to the
	// resource id, while on forwarded List operations the relationship filter
	// is preset.
	//
	// Usage: Read Only
	// Availability: Authorizers
	// Operations: !Create, !CollectionAction
	Selector bson.M

	// The filters that will be used during a List, Find, Update, Delete or
	// ResourceAction operation to further filter the selection of a list of
	// models or a specific model.
	//
	// On List operations, attribute and relationship filters are preset.
	//
	// Usage: Append Only
	// Availability: Authorizers
	// Operations: !Create, !CollectionAction
	Filters []bson.M

	// The sorting that will be used during List.
	//
	// Usage: No Restriction
	// Availability: Authorizers
	// Operations: List
	Sorting []string

	// Only the whitelisted readable fields are exposed to the client as
	// attributes and relationships. Additionally, only readable fields can
	// be used for filtering and sorting.
	//
	// Usage: Reduce Only
	// Availability: Authorizers
	// Operations: !Delete, !ResourceAction, !CollectionAction
	ReadableFields []string

	// Used instead of ReadableFields if set. Allows specifying readable fields
	// on a per model basis. The provided model may be nil if run before any
	// model has been loaded.
	GetReadableFields func(coal.Model) []string

	// Only the whitelisted writable fields can be altered by requests.
	//
	// Usage: Reduce Only
	// Availability: Authorizers
	// Operations: Create, Update
	WritableFields []string

	// Used instead of WritableFields if set. Allows specifying writable fields
	// on a per model basis. The provided model may be nil if run before any
	// model has been loaded.
	GetWritableFields func(coal.Model) []string

	// Only the whitelisted readable properties are exposed to the client as
	// attributes.
	//
	// Usage: Reduce Only
	// Availability: Authorizers
	// Operations: !Delete, !ResourceActon, !CollectionAction
	ReadableProperties []string

	// Used instead of ReadableProperties if set. Allows specifying readable
	// properties on a per model basis. The provided model may be nil if run
	// before any model has been loaded.
	GetReadableProperties func(coal.Model) []string

	// The filters that will be applied when loading has one and has many
	// relationships.
	//
	// Usage: Append Only
	// Availability: Authorizers.
	// Operations: !Create, !CollectionAction
	RelationshipFilters map[string][]bson.M

	// The model that will be created, updated, deleted or is requested by a
	// resource action.
	//
	// Usage: Modify Only
	// Availability: Validators
	// Operations: Create, Update, Delete, ResourceAction
	Model coal.Model

	// The models that will be returned for a List operation.
	//
	// Usage: Modify Only
	// Availability: Decorators
	// Operations: List
	Models []coal.Model

	// The original model that is being updated. Can be used to lookup up
	// original values of changed fields.
	//
	// Usage: Ready Only
	// Availability: Validators
	// Operations: Update
	Original coal.Model

	// The document that has been received by the client.
	//
	// Usage: Read Only
	// Availability: Authorizers
	// Operations: !List, !Find, !CollectionAction, !ResourceAction
	Request *jsonapi.Document

	// The document that will be written to the client.
	//
	// Usage: Modify Only
	// Availability: Notifiers
	// Operations: !CollectionAction, !ResourceAction
	Response *jsonapi.Document

	// The status code that will be written to the client.
	//
	// Usage: Modify Only
	// Availability: Notifiers
	// Operations: !CollectionAction, !ResourceAction
	ResponseCode int

	// The store that is used to retrieve and persist the model.
	//
	// Usage: Read Only
	Store *coal.Store

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

	// The controller that is managing the request.
	//
	// Usage: Read Only
	Controller *Controller

	// The group that received the request.
	//
	// Usage: Read Only
	Group *Group

	// The current tracer.
	//
	// Usage: Read Only
	Tracer *xo.Tracer
}

// With will run the provided function with the specified context temporarily
// set on the context. This is especially useful together with transactions.
func (c *Context) With(ctx context.Context, fn func()) {
	// retain current context and tracer
	oc := c.Context
	ot := c.Tracer

	// swap tracer and context
	if c.Tracer != nil {
		c.Tracer, c.Context = xo.NewTracer(ctx)
	} else {
		c.Context = ctx
	}

	// yield
	fn()

	// switchback
	c.Context = oc
	c.Tracer = ot
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

	// otherwise, return $and query
	return bson.M{"$and": subQueries}
}

// Modified will return whether the specified field has been changed. During an
// update operation the modification is checked against the original model. For
// all other operations, the field is checked against its zero value.
func (c *Context) Modified(field string) bool {
	// determine old value
	var oldValue interface{}
	if c.Original != nil {
		oldValue = stick.MustGet(c.Original, field)
	} else {
		oldValue = reflect.Zero(coal.GetMeta(c.Model).Fields[field].Type).Interface()
	}

	// get new value
	newValue := stick.MustGet(c.Model, field)

	return !reflect.DeepEqual(newValue, oldValue)
}

// Parse will decode a custom JSON body to the specified object.
func (c *Context) Parse(obj interface{}) error {
	// unmarshal json
	err := json.NewDecoder(c.HTTPRequest.Body).Decode(obj)
	if err == io.EOF {
		return xo.SF("incomplete request body")
	} else if err != nil {
		return xo.W(err)
	}

	return nil
}

// Respond will encode the provided value as JSON and write it to the client.
func (c *Context) Respond(value interface{}) error {
	// encode response
	bytes, err := json.Marshal(value)
	if err != nil {
		return xo.W(err)
	}

	// write token
	_, err = c.ResponseWriter.Write(bytes)
	if err != nil {
		return xo.W(err)
	}

	return nil
}
