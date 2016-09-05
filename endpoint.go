package fire

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// An Endpoint mounts and provides access to multiple resources.
type Endpoint struct {
	db          *mgo.Database
	prefix      string
	nameMap     map[string]string
	resourceMap map[string]*Resource
	resources   []*Resource
}

// NewEndpoint returns a new fire endpoint.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewEndpoint(db *mgo.Database, prefix string) *Endpoint {
	return &Endpoint{
		db:          db,
		prefix:      prefix,
		nameMap:     make(map[string]string),
		resourceMap: make(map[string]*Resource),
	}
}

// AddResource will add a resource to the API endpoint.
//
// Note: Each resource should only be added once.
func (e *Endpoint) AddResource(resource *Resource) {
	// initialize model
	Init(resource.Model)

	// create entry in name map
	e.nameMap[resource.Model.Meta().PluralName] = resource.Model.Meta().SingularName

	// create entry in resource map
	e.resourceMap[resource.Model.Meta().SingularName] = resource

	// add resource to internal list
	resource.endpoint = e
	e.resources = append(e.resources, resource)
}

// Register will create all necessary routes on the passed router.
//
// Note: This function should only be called once.
func (e *Endpoint) Register(router gin.IRouter) {
	// process all resources
	for _, r := range e.resources {
		pluralName := r.Model.Meta().PluralName

		// add basic operations
		router.GET(e.prefix+"/"+pluralName, r.generalHandler)
		router.POST(e.prefix+"/"+pluralName, r.generalHandler)
		router.GET(e.prefix+"/"+pluralName+"/:id", r.generalHandler)
		router.PATCH(e.prefix+"/"+pluralName+"/:id", r.generalHandler)
		router.DELETE(e.prefix+"/"+pluralName+"/:id", r.generalHandler)

		// process all relationships
		for _, field := range r.Model.Meta().Fields {
			if field.RelName == "" {
				continue
			}

			name := field.RelName

			// add relationship queries
			router.GET(e.prefix+"/"+pluralName+"/:id/"+name, r.generalHandler)
			router.GET(e.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)

			// add relationship management operations
			if field.ToOne || field.ToMany {
				router.PATCH(e.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)
			}

			if field.ToMany {
				router.POST(e.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)
				router.DELETE(e.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)
			}
		}
	}
}
