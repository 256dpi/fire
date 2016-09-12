package fire

import (
	"fmt"
	"sort"
	"time"

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
	devMode    bool
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

// EnableDevMode will enable the development mode that prints all registered
// handlers on boot and all incoming requests.
func (a *Application) EnableDevMode() {
	a.devMode = true
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) {
	a.run(standard.New(addr))
}

// SecureStart will run the application on the specified address using a TLS
// certificate.
func (a *Application) SecureStart(addr, certFile, keyFile string) {
	a.run(standard.WithTLS(addr, certFile, keyFile))
}

func (a *Application) run(server engine.Server) {
	// create new router
	router := echo.New()

	// register components
	for _, component := range a.components {
		component.Register(router)
	}

	// enable dev mode
	if a.devMode {
		a.printDevInfo(router)
		router.Use(a.logger)
	}

	// run router
	router.Run(server)
}

func (a *Application) printDevInfo(router *echo.Echo) {
	// return if not enabled
	if !a.devMode {
		return
	}

	// print header
	fmt.Println("==> Fire application starting...")
	fmt.Println("==> Registered routes:")

	// prepare routes
	var routes []string

	// add all routes as string
	for _, route := range router.Routes() {
		routes = append(routes, fmt.Sprintf("%6s  %-30s", route.Method, route.Path))
	}

	// sort routes
	sort.Strings(routes)

	// print routes
	for _, route := range routes {
		fmt.Println(route)
	}

	// print footer
	fmt.Println("==> Ready to go!")
}

func (a *Application) logger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		req := c.Request()
		res := c.Response()

		// save start
		start := time.Now()

		// call next handler
		if err = next(c); err != nil {
			c.Error(err)
		}

		// get request duration
		duration := time.Since(start).String()

		// fix path
		path := req.URL().Path()
		if path == "" {
			path = "/"
		}

		// log request
		fmt.Printf("%6s  %-30s  %d  %s\n", req.Method(), path, res.Status(), duration)

		return
	}
}
