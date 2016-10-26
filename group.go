package fire

import (
	"net/http"
	"strings"

	"github.com/gonfire/jsonapi"
)

// A Group manages access to multiple controllers and their interconnections.
type Group struct {
	controllers map[string]*Controller
}

// NewGroup creates and returns a new group.
func NewGroup() *Group {
	return &Group{
		controllers: make(map[string]*Controller),
	}
}

// Add will add a controller to the group.
func (g *Group) Add(controllers ...*Controller) {
	for _, controller := range controllers {
		// initialize model
		Init(controller.Model)

		// create entry in controller map
		g.controllers[controller.Model.Meta().PluralName] = controller
	}
}

// Endpoint will return an http handler that serves requests for this controller
// group. The specified prefix is used to parse the requests and generate urls
// for the resources.
func (g *Group) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// trim and split path
		s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")

		// try to call the controllers general handler
		if len(s) > 0 {
			if controller, ok := g.controllers[s[0]]; ok {
				controller.generalHandler(g, prefix, w, r)
				return
			}
		}

		// write not found error
		jsonapi.WriteError(w, jsonapi.NotFound("Resource not found"))
	})
}
