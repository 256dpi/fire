// Package fire is an idiomatic micro-framework for building Ember.js compatible
// APIs with Go.
package fire

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

// GroupAction defines a group action.
type GroupAction struct {
	// Authorizers authorize the group action and are run before the action.
	// Returned errors will cause the abortion of the request with an
	// "Unauthorized" status by default.
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

// Add will add controllers to the group.
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

// Handle allows to add an action as a group action. Group actions will only be
// run when no controller matches the request.
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

// Endpoint will return a handler that serves requests for this group. The
// specified prefix is used to parse the requests and generate URLs for the
// resources.
func (g *Group) Endpoint(prefix string) http.Handler {
	// trim prefix
	prefix = strings.Trim(prefix, "/")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// create tracer
		tracer, tc := xo.CreateTracer(r.Context(), "fire/Group.Endpoint")
		defer tracer.End()
		r = r.WithContext(tc)

		// recover any panic
		defer xo.Recover(func(err error) {
			// record error
			tracer.Record(err)

			// report error if possible or rethrow
			if g.reporter != nil {
				g.reporter(err)
			}

			// write internal server error
			_ = jsonapi.WriteError(w, jsonapi.InternalServerError(""))
		})

		// continue any previous aborts
		defer xo.Resume(func(err error) {
			// directly write jsonapi errors
			var jsonapiError *jsonapi.Error
			if errors.As(err, &jsonapiError) {
				_ = jsonapi.WriteError(w, jsonapiError)
				return
			}

			// record error
			tracer.Record(err)

			// report error if possible
			if g.reporter != nil {
				g.reporter(err)
			}

			// write internal server error
			_ = jsonapi.WriteError(w, jsonapi.InternalServerError(""))
		})

		// trim path
		path := strings.Trim(r.URL.Path, "/")
		path = strings.TrimPrefix(path, prefix)
		path = strings.Trim(path, "/")

		// check path
		if path == "" {
			xo.Abort(jsonapi.NotFound("resource not found"))
		}

		// split path
		s := strings.Split(path, "/")

		// prepare context
		ctx := &Context{
			Context:        r.Context(),
			Data:           stick.Map{},
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
			if stick.Contains(action.Action.Methods, r.Method) {
				// run authorizers and handle errors
				for _, cb := range action.Authorizers {
					// check if callback should be run
					if !cb.Matcher(ctx) {
						continue
					}

					// call callback
					err := cb.Handler(ctx)
					if xo.IsSafe(err) {
						xo.Abort(&jsonapi.Error{
							Status: http.StatusUnauthorized,
							Detail: err.Error(),
						})
					} else if err != nil {
						xo.Abort(err)
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
				xo.AbortIf(action.Action.Handler(ctx))

				return
			}
		}

		// otherwise, return error
		xo.Abort(jsonapi.NotFound("resource not found"))
	})
}
