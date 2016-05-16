package fire

import (
	"github.com/gin-gonic/gin"
	"github.com/manyminds/api2go"
	"gopkg.in/mgo.v2"
)

// An Endpoint provides access to multiple resources.
type Endpoint struct {
	db        *mgo.Database
	nameMap   map[string]string
	resources []*Resource
}

// NewEndpoint returns a new fire endpoint.
func NewEndpoint(db *mgo.Database) *Endpoint {
	return &Endpoint{
		db:      db,
		nameMap: make(map[string]string),
	}
}

// AddResource will add a resource to the API endpoint.
//
// Note: Each resource should only be added once.
func (e *Endpoint) AddResource(resource *Resource) {
	// initialize model
	Init(resource.Model)

	// create entry in name map
	e.nameMap[resource.Model.GetName()] = resource.Model.getBase().singularName

	// add resource to internal list
	resource.endpoint = e
	e.resources = append(e.resources, resource)
}

// Register will create all necessary routes on the passed router. If want to
// prefix your api (e.g. /api/) you need to pass it to Register. This is
// necessary for generating the proper links in the JSON documents.
//
// Note: This functions should only be called once after registering all resources.
func (e *Endpoint) Register(prefix string, router gin.IRouter) {
	// create gin adapter
	adapter := newAdapter(router)

	// create routing configuration using the custom gin adapter
	api := api2go.NewAPIWithRouting(
		prefix,
		api2go.NewStaticResolver("/"),
		adapter,
	)

	// process all resources
	for _, resource := range e.resources {
		// assign adapter
		resource.adapter = adapter

		// add resource to api
		api.AddResource(resource.Model, resource)
	}
}
