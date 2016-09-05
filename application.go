package fire

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// An Application mounts and manages access to multiple controllers.
type Application struct {
	db            *mgo.Database
	prefix        string
	nameMap       map[string]string
	controllerMap map[string]*Controller
	controllers   []*Controller
}

// NewApplication returns a new fire application.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewApplication(db *mgo.Database, prefix string) *Application {
	return &Application{
		db:            db,
		prefix:        prefix,
		nameMap:       make(map[string]string),
		controllerMap: make(map[string]*Controller),
	}
}

// Mount will add a controller to the application.
//
// Note: Each controller should only be mounted once.
func (a *Application) Mount(controller *Controller) {
	// initialize model
	Init(controller.Model)

	// create entry in name map
	a.nameMap[controller.Model.Meta().PluralName] = controller.Model.Meta().SingularName

	// create entry in controller map
	a.controllerMap[controller.Model.Meta().SingularName] = controller

	// add controller to internal list
	controller.app = a
	a.controllers = append(a.controllers, controller)
}

// Register will create all necessary routes on the passed router.
//
// Note: This function should only be called once.
func (a *Application) Register(router gin.IRouter) {
	// process all controllers
	for _, r := range a.controllers {
		pluralName := r.Model.Meta().PluralName

		// add basic operations
		router.GET(a.prefix+"/"+pluralName, r.generalHandler)
		router.POST(a.prefix+"/"+pluralName, r.generalHandler)
		router.GET(a.prefix+"/"+pluralName+"/:id", r.generalHandler)
		router.PATCH(a.prefix+"/"+pluralName+"/:id", r.generalHandler)
		router.DELETE(a.prefix+"/"+pluralName+"/:id", r.generalHandler)

		// process all relationships
		for _, field := range r.Model.Meta().Fields {
			if field.RelName == "" {
				continue
			}

			name := field.RelName

			// add relationship queries
			router.GET(a.prefix+"/"+pluralName+"/:id/"+name, r.generalHandler)
			router.GET(a.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)

			// add relationship management operations
			if field.ToOne || field.ToMany {
				router.PATCH(a.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)
			}

			if field.ToMany {
				router.POST(a.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)
				router.DELETE(a.prefix+"/"+pluralName+"/:id/relationships/"+name, r.generalHandler)
			}
		}
	}
}
