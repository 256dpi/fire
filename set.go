package fire

import (
	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
)

// A Set manages access to multiple controllers and their interconnections.
type Set struct {
	db          *mgo.Database
	router      *echo.Echo
	prefix      string
	controllers map[string]*Controller
}

// NewSet returns a new controller set.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewSet(db *mgo.Database, router *echo.Echo, prefix string) *Set {
	return &Set{
		db:          db,
		router:      router,
		prefix:      prefix,
		controllers: make(map[string]*Controller),
	}
}

// Mount will add controllers to the set and register them on the router.
//
// Note: Each controller should only be mounted once.
func (s *Set) Mount(controllers ...*Controller) {
	for _, controller := range controllers {
		// initialize model
		Init(controller.Model)

		// create entry in controller map
		s.controllers[controller.Model.Meta().PluralName] = controller

		// set reference on controller
		controller.set = s

		// register controller
		controller.register(s.router, s.prefix)
	}
}
