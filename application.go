package fire

import (
	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
)

// An Application mounts and manages access to multiple controllers.
type Application struct {
	db          *mgo.Database
	prefix      string
	controllers map[string]*Controller
}

// New returns a new fire application.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func New(db *mgo.Database, prefix string) *Application {
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
func (a *Application) Register(router *echo.Echo) {
	// process all controllers
	for _, c := range a.controllers {
		c.register(router, a.prefix)
	}
}
