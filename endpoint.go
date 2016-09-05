package fire

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// An Endpoint mounts and manages access to multiple controllers.
type Endpoint struct {
	db            *mgo.Database
	prefix        string
	nameMap       map[string]string
	controllerMap map[string]*Controller
	controllers   []*Controller
}

// NewEndpoint returns a new fire endpoint.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewEndpoint(db *mgo.Database, prefix string) *Endpoint {
	return &Endpoint{
		db:            db,
		prefix:        prefix,
		nameMap:       make(map[string]string),
		controllerMap: make(map[string]*Controller),
	}
}

// Mount will add a controller to the endpoint.
//
// Note: Each controller should only be mounted once.
func (e *Endpoint) Mount(controller *Controller) {
	// initialize model
	Init(controller.Model)

	// create entry in name map
	e.nameMap[controller.Model.Meta().PluralName] = controller.Model.Meta().SingularName

	// create entry in controller map
	e.controllerMap[controller.Model.Meta().SingularName] = controller

	// add controller to internal list
	controller.endpoint = e
	e.controllers = append(e.controllers, controller)
}

// Register will create all necessary routes on the passed router.
//
// Note: This function should only be called once.
func (e *Endpoint) Register(router gin.IRouter) {
	// process all controllers
	for _, r := range e.controllers {
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
