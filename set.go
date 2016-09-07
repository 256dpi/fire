package fire

import (
	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
)

// A Set manages access to multiple controllers and their interconnections.
type Set struct {
	session     *mgo.Session
	router      *echo.Echo
	prefix      string
	controllers map[string]*Controller
}

// NewSet creates and returns a new set.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewSet(session *mgo.Session, router *echo.Echo, prefix string) *Set {
	return &Set{
		session:     session,
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

func (s *Set) sessionAndDatabase() (*mgo.Session, *mgo.Database) {
	sess := s.session.Clone()
	return sess, sess.DB("")
}
