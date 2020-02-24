// Package fire is an idiomatic micro-framework for building Ember.js compatible
// APIs with Go.
package fire

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/stack"

	"github.com/256dpi/fire/coal"
)

// GroupAction defines a group action.
type GroupAction struct {
	// Authorizers authorize the group action and are run before the action.
	// Returned errors will cause the abortion of the request with an
	// unauthorized status by default.
	Authorizers []*Callback

	// Action is the action that should be executed.
	Action *Action
}

// A Group manages access to multiple controllers and their interconnections.
type Group struct {
	reporter    func(error)
	controllers map[string]*Controller
	actions     map[string]*GroupAction
}

// NewGroup creates and returns a new group.
func NewGroup(reporter func(error)) *Group {
	return &Group{
		reporter:    reporter,
		controllers: make(map[string]*Controller),
		actions:     make(map[string]*GroupAction),
	}
}

// Add will add a controller to the group.
func (g *Group) Add(controllers ...*Controller) {
	for _, controller := range controllers {
		// prepare controller
		controller.prepare()

		// get name
		name := coal.GetMeta(controller.Model).PluralName

		// check existence
		if g.controllers[name] != nil {
			panic(fmt.Sprintf(`fire: controller with name "%s" already exists`, name))
		}

		// create entry in controller map
		g.controllers[name] = controller
	}
}

// Handle allows to add an action as a group action. Group actions will be run
// when no controller matches the request.
//
// Note: The passed context is more or less empty.
func (g *Group) Handle(name string, a *GroupAction) {
	if name == "" {
		panic(fmt.Sprintf(`fire: invalid group action "%s"`, name))
	}

	// set default body limit
	if a.Action.BodyLimit == 0 {
		a.Action.BodyLimit = serve.MustByteSize("8M")
	}

	// set default timeout
	if a.Action.Timeout == 0 {
		a.Action.Timeout = 30 * time.Second
	}

	// check existence
	if g.actions[name] != nil {
		panic(fmt.Sprintf(`fire: action with name "%s" already exists`, name))
	}

	// add action
	g.actions[name] = a
}

// Endpoint will return an http handler that serves requests for this group. The
// specified prefix is used to parse the requests and generate urls for the
// resources.
func (g *Group) Endpoint(prefix string) http.Handler {
	// trim prefix
	prefix = strings.Trim(prefix, "/")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// create tracer
		tracer := NewTracerFromRequest(r, "fire/Group.Endpoint")
		defer tracer.Finish(true)

		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write jsonapi errors
			if jsonapiError, ok := err.(*jsonapi.Error); ok {
				_ = jsonapi.WriteError(w, jsonapiError)
				return
			}

			// set critical error on last span
			tracer.Tag("error", true)
			tracer.Log("error", err.Error())
			tracer.Log("stack", stack.Trace())

			// report critical errors if possible
			if g.reporter != nil {
				g.reporter(err)
			}

			// ignore errors caused by writing critical errors
			_ = jsonapi.WriteError(w, jsonapi.InternalServerError(""))
		})

		// trim path
		path := strings.Trim(r.URL.Path, "/")
		path = strings.TrimPrefix(path, prefix)
		path = strings.Trim(path, "/")

		// check path
		if path == "" {
			stack.Abort(jsonapi.NotFound("resource not found"))
		}

		// split path
		s := strings.Split(path, "/")

		// prepare context
		ctx := &Context{
			Context:        r.Context(),
			Data:           Map{},
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

			// handle request
			controller.handle(prefix, ctx, nil, true)

			return
		}

		// get action
		action, ok := g.actions[s[0]]
		if ok {
			// check if action is allowed
			if Contains(action.Action.Methods, r.Method) {
				// run authorizers and handle errors
				for _, cb := range action.Authorizers {
					// check if callback should be run
					if !cb.Matcher(ctx) {
						continue
					}

					// call callback
					err := cb.Handler(ctx)
					if IsSafe(err) {
						stack.Abort(&jsonapi.Error{
							Status: http.StatusUnauthorized,
							Detail: err.Error(),
						})
					} else if err != nil {
						stack.Abort(err)
					}
				}

				// limit request body size
				serve.LimitBody(ctx.ResponseWriter, ctx.HTTPRequest, action.Action.BodyLimit)

				// create context
				ct, cancel := context.WithTimeout(ctx.Context, action.Action.Timeout)
				defer cancel()

				// replace context
				ctx.Context = ct

				// call action with context
				stack.AbortIf(action.Action.Handler(ctx))

				return
			}
		}

		// otherwise return error
		stack.Abort(jsonapi.NotFound("resource not found"))
	})
}
