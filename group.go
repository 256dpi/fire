package fire

import (
	"net/http"
	"strings"

	"github.com/256dpi/jsonapi"
	"github.com/256dpi/stack"
)

// A Group manages access to multiple controllers and their interconnections.
type Group struct {
	controllers map[string]*Controller

	// The function gets invoked by the controller with occurring fatal errors.
	Reporter func(error)

	// The function that should be used to log debugging information.
	Logger func(string)
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
		// prepare controller
		controller.prepare()

		// get name
		name := controller.Model.Meta().PluralName

		// create entry in controller map
		g.controllers[name] = controller
	}
}

// List will return a list of all added controllers.
func (g *Group) List() []*Controller {
	list := make([]*Controller, 0, len(g.controllers))

	for _, c := range g.controllers {
		list = append(list, c)
	}

	return list
}

// Find will return the controller with the matching plural name if found.
func (g *Group) Find(pluralName string) *Controller {
	return g.controllers[pluralName]
}

// Endpoint will return an http handler that serves requests for this controller
// group. The specified prefix is used to parse the requests and generate urls
// for the resources.
func (g *Group) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write potential bearer errors
			if jsonapiError, ok := err.(*jsonapi.Error); ok {
				jsonapi.WriteError(w, jsonapiError)
				return
			}

			// otherwise report critical errors
			if g.Reporter != nil {
				g.Reporter(err)
			}

			// ignore errors caused by writing critical errors
			jsonapi.WriteError(w, jsonapi.InternalServerError(""))
		})

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
		jsonapi.WriteError(w, jsonapi.NotFound("resource not found"))
	})
}
