// Package fire implements a small and opinionated framework for Go providing
// Ember compatible JSON APIs.
package fire

import (
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
)

// Map is a general purpose map used for configuration.
type Map map[string]interface{}

// A Component can be mounted on an application.
type Component interface {
	// Register will be called by the application with a new echo router.
	Register(router *echo.Echo)

	// Inspect will be called by the Inspector to print a list of used
	// components and their configuration.
	Inspect() ComponentInfo
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
}

// New creates and returns a new Application.
func New() *Application {
	return &Application{}
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
	// create new router
	router := echo.New()

	// prepare inspector
	var inspector *Inspector

	// register components
	for _, component := range a.components {
		component.Register(router)

		// set inspector when available
		if i, ok := component.(*Inspector); ok {
			inspector = i
		}
	}

	// signal boot
	if inspector != nil {
		inspector.boot()
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

	// signal run
	if inspector != nil {
		inspector.inspect(a, router)
	}

	// signal run
	if inspector != nil {
		inspector.run()
	}

	// run router
	router.Run(server)

	// signal run
	if inspector != nil {
		inspector.teardown()
	}

	// teardown components
	for _, component := range a.components {
		if bootable, ok := component.(BootableComponent); ok {
			err := bootable.Teardown()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
