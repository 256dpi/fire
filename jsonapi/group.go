package jsonapi

import (
	"strings"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
)

// A Group manages access to multiple controllers and their interconnections.
type Group struct {
	prefix      string
	controllers map[string]*Controller
}

// New creates and returns a new group.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func New(prefix string) *Group {
	return &Group{
		prefix:      prefix,
		controllers: make(map[string]*Controller),
	}
}

// Add will add a controller to the group.
func (g *Group) Add(controllers ...*Controller) {
	for _, controller := range controllers {
		// initialize model
		model.Init(controller.Model)

		// create entry in controller map
		g.controllers[controller.Model.Meta().PluralName] = controller

		// set reference on controller
		controller.group = g
	}
}

// Register implements the Component interface.
func (g *Group) Register(router *echo.Echo) {
	for _, controller := range g.controllers {
		controller.register(router, g.prefix)
	}
}

// Inspect implements the InspectableComponent interface.
func (g *Group) Inspect() fire.ComponentInfo {
	// prepare resource names
	var names []string

	// add model names
	for _, controller := range g.controllers {
		names = append(names, controller.Model.Meta().PluralName)
	}

	return fire.ComponentInfo{
		Name: "JSON API Group",
		Settings: fire.Map{
			"Prefix":    g.prefix,
			"Resources": strings.Join(names, ", "),
		},
	}
}
