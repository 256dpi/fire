package fire

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
)

// A Component can be mounted on an application.
type Component interface {
	Register(router *echo.Echo)
}

// An Application provides an out-of-the-box configuration of components to
// get started with building JSON APIs.
type Application struct {
	components []Component
}

// New creates and returns a new Application.
func New() *Application {
	return &Application{}
}

// Mount will mount the passed Component in the application using the passed
// prefix.
//
// Note: Each component should only be mounted once before calling Run or Start.
func (a *Application) Mount(component Component) {
	a.components = append(a.components, component)
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) {
	a.Run(standard.New(addr))
}

// SecureStart will run the application on the specified address using a TLS
// certificate.
func (a *Application) SecureStart(addr, certFile, keyFile string) {
	a.Run(standard.WithTLS(addr, certFile, keyFile))
}

// Run will run the application using the specified server.
func (a *Application) Run(server engine.Server) {
	// create new router
	router := echo.New()

	// register components
	for _, component := range a.components {
		component.Register(router)
	}

	// run router
	router.Run(server)
}
