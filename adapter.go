package fire

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/manyminds/api2go"
	"github.com/manyminds/api2go/routing"
)

// implements the `api2go/routing.Routeable` interface
type adapter struct {
	router        gin.IRoutes
	contexts      map[*http.Request]*gin.Context
	contextsMutex sync.Mutex
}

func newAdapter(router gin.IRouter) *adapter {
	return &adapter{
		router:   router,
		contexts: make(map[*http.Request]*gin.Context),
	}
}

func (a *adapter) Handler() http.Handler {
	return nil
}

func (a *adapter) Handle(protocol, route string, handler routing.HandlerFunc) {
	// register adapted handler
	a.router.Handle(protocol, route, func(ctx *gin.Context) {
		// extract all params
		params := map[string]string{}
		for _, p := range ctx.Params {
			params[p.Key] = p.Value
		}

		// save context
		a.contextsMutex.Lock()
		a.contexts[ctx.Request] = ctx
		a.contextsMutex.Unlock()

		// call api2go handler
		handler(ctx.Writer, ctx.Request, params)

		// remove context again
		a.contextsMutex.Lock()
		delete(a.contexts, ctx.Request)
		a.contextsMutex.Unlock()
	})
}

func (a *adapter) getContext(req *api2go.Request) *gin.Context {
	a.contextsMutex.Lock()
	defer a.contextsMutex.Unlock()

	return a.contexts[req.PlainRequest]
}
