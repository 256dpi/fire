package jsonapi

import (
	"net/http"
	"strings"

	"github.com/gonfire/fire/model"
	"github.com/gonfire/jsonapi"
)

// A Group manages access to multiple controllers and their interconnections.
type Group struct {
	prefix      string
	controllers map[string]*Controller
}

// NewGroup creates and returns a new group.
//
// Note: You should pass the full URL prefix of the API to allow proper
// generation of resource links.
func NewGroup(prefix string) *Group {
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

func (g *Group) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// trim and split path
	s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, g.prefix), "/"), "/")

	// try to call the controllers general handler
	if len(s) > 0 {
		if controller, ok := g.controllers[s[0]]; ok {
			controller.ServeHTTP(w, r)
			return
		}
	}

	// write not found error
	jsonapi.WriteError(w, jsonapi.NotFound("Resource not found"))
}
