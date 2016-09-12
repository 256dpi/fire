package fire

import (
	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
)

// A ControllerGroup manages access to multiple controllers and their interconnections.
type ControllerGroup struct {
	session     *mgo.Session
	prefix      string
	controllers map[string]*Controller
}

// NewControllerGroup creates and returns a new controller group.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewControllerGroup(session *mgo.Session, prefix string) *ControllerGroup {
	return &ControllerGroup{
		session:     session,
		prefix:      prefix,
		controllers: make(map[string]*Controller),
	}
}

// Add will add a controller to the group.
func (g *ControllerGroup) Add(controllers ...*Controller) {
	for _, controller := range controllers {
		// initialize model
		Init(controller.Model)

		// create entry in controller map
		g.controllers[controller.Model.Meta().PluralName] = controller

		// set reference on controller
		controller.group = g
	}
}

// Register will register the controller group on the passed echo instance.
func (g *ControllerGroup) Register(router *echo.Echo) {
	for _, controller := range g.controllers {
		controller.register(router, g.prefix)
	}
}

func (g *ControllerGroup) sessionAndDatabase() (*mgo.Session, *mgo.Database) {
	sess := g.session.Clone()
	return sess, sess.DB("")
}
