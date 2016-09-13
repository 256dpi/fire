package fire

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
)

// A Component can be mounted on an application.
type Component interface {
	// Register will be called by the application with a new echo router.
	Register(router *echo.Echo)
}

// A BootableComponent is an extended component with additional methods for
// setup and teardown.
type BootableComponent interface {
	Component

	// Setup will be called before the applications starts and allows further
	// initialization.
	Setup() error

	// Teardown will be called after applications has stopped and allows proper
	// cleanup.
	Teardown() error
}

// An Application provides a simple way to combine multiple components.
type Application struct {
	components []Component
	router     *echo.Echo
}

// New creates and returns a new Application.
func New() *Application {
	return &Application{
		router: echo.New(),
	}
}

// Mount will mount the passed Component in the application.
//
// Note: Each component should only be mounted once before calling Run or Start.
func (a *Application) Mount(component Component) {
	a.components = append(a.components, component)
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) error {
	return a.Run(standard.New(addr))
}

// SecureStart will run the application on the specified address using a TLS
// certificate.
func (a *Application) SecureStart(addr, certFile, keyFile string) error {
	return a.Run(standard.WithTLS(addr, certFile, keyFile))
}

// Run will run the application using the specified server.
func (a *Application) Run(server engine.Server) error {
	// register components
	for _, component := range a.components {
		component.Register(a.router)
	}

	// setup components
	for _, component := range a.components {
		if bootable, ok := component.(BootableComponent); ok {
			err := bootable.Setup()
			if err != nil {
				return err
			}
		}
	}

	// run router
	a.router.Run(server)

	// teardown components
	for _, component := range a.components {
		if bootable, ok := component.(BootableComponent); ok {
			err := bootable.Teardown()
			if err != nil {
				return err
			}
		}
	}

	// remove router when server stops
	a.router = nil

	return nil
}
