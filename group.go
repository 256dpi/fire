// Package fire is an idiomatic micro-framework for building Ember.js compatible
// APIs with Go.
package fire

import (
	"net/http"
	"strings"

	"github.com/256dpi/jsonapi"
	"github.com/256dpi/stack"
)

// TODO: Pull in CORS from wood an make it a first class citizen.

// A Group manages access to multiple controllers and their interconnections.
type Group struct {
	controllers map[string]*Controller
	actions     map[string]*Action

	// The function gets invoked by the controller with critical errors.
	Reporter func(error)
}

// NewGroup creates and returns a new group.
func NewGroup() *Group {
	return &Group{
		controllers: make(map[string]*Controller),
		actions:     make(map[string]*Action),
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

// Handle allows to add an action as a group action. Group actions will be run
// when no controller matches the request.
//
// Note: The passed context is more or less empty.
func (g *Group) Handle(name string, a *Action) {
	// set default body limit
	if a.BodyLimit == 0 {
		a.BodyLimit = DataSize("8M")
	}

	// add action
	g.actions[name] = a
}

// Endpoint will return an http handler that serves requests for this group. The
// specified prefix is used to parse the requests and generate urls for the
// resources.
func (g *Group) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// create tracer
		tracer := NewTracerFromRequest(r, "fire/Group.Endpoint")
		defer tracer.Finish(true)

		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write jsonapi errors
			if jsonapiError, ok := err.(*jsonapi.Error); ok {
				jsonapi.WriteError(w, jsonapiError)
				return
			}

			// set critical error on last span
			tracer.Tag("error", true)
			tracer.Log("error", err.Error())
			tracer.Log("stack", stack.Trace())

			// report critical errors if possible
			if g.Reporter != nil {
				g.Reporter(err)
			}

			// ignore errors caused by writing critical errors
			jsonapi.WriteError(w, jsonapi.InternalServerError(""))
		})

		// trim and split path
		s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")

		// check segments
		if len(s) == 0 {
			stack.Abort(jsonapi.NotFound("resource not found"))
		}

		// prepare context
		ctx := &Context{
			HTTPRequest:    r,
			ResponseWriter: w,
			Group:          g,
			Tracer:         tracer,
		}

		// get controller
		controller, ok := g.controllers[s[0]]
		if ok {
			// set controller
			ctx.Controller = controller

			// call controller with context
			controller.generalHandler(prefix, ctx)

			return
		}

		// get action
		action, ok := g.actions[s[0]]
		if ok {
			// check if action is allowed
			if Contains(action.Methods, r.Method) {
				// check if action matches the context
				if action.Callback.Matcher(ctx) {
					// limit request body size
					ctx.HTTPRequest.Body = http.MaxBytesReader(ctx.ResponseWriter, ctx.HTTPRequest.Body, int64(action.BodyLimit))

					// call action with context
					stack.AbortIf(action.Callback.Handler(ctx))

					return
				}
			}
		}

		// otherwise return error
		stack.Abort(jsonapi.NotFound("resource not found"))
	})
}
