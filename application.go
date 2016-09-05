package fire

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

// An Application mounts and manages access to multiple controllers.
type Application struct {
	db          *mgo.Database
	prefix      string
	controllers map[string]*Controller
}

// NewApplication returns a new fire application.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewApplication(db *mgo.Database, prefix string) *Application {
	return &Application{
		db:          db,
		prefix:      prefix,
		controllers: make(map[string]*Controller),
	}
}

// Mount will add controllers to the application.
//
// Note: Each controller should only be mounted once.
func (a *Application) Mount(controllers ...*Controller) {
	for _, controller := range controllers {
		// initialize model
		Init(controller.Model)

		// create entry in controller map
		a.controllers[controller.Model.Meta().PluralName] = controller

		// set app on controller
		controller.app = a
	}
}

// Register will create all necessary routes on the passed router.
//
// Note: This function should only be called once.
func (a *Application) Register(router gin.IRouter) {
	// process all controllers
	for _, c := range a.controllers {
		pluralName := c.Model.Meta().PluralName

		// add basic operations
		router.GET(a.prefix+"/"+pluralName, c.generalHandler)
		router.POST(a.prefix+"/"+pluralName, c.generalHandler)
		router.GET(a.prefix+"/"+pluralName+"/:id", c.generalHandler)
		router.PATCH(a.prefix+"/"+pluralName+"/:id", c.generalHandler)
		router.DELETE(a.prefix+"/"+pluralName+"/:id", c.generalHandler)

		// process all relationships
		for _, field := range c.Model.Meta().Fields {
			if field.RelName == "" {
				continue
			}

			name := field.RelName

			// add relationship queries
			router.GET(a.prefix+"/"+pluralName+"/:id/"+name, c.generalHandler)
			router.GET(a.prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)

			// add relationship management operations
			if field.ToOne || field.ToMany {
				router.PATCH(a.prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)
			}

			if field.ToMany {
				router.POST(a.prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)
				router.DELETE(a.prefix+"/"+pluralName+"/:id/relationships/"+name, c.generalHandler)
			}
		}
	}
}
