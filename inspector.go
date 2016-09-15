package fire

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/labstack/echo"
)

// A ComponentInfo is returned by a component to describe itself.
type ComponentInfo struct {
	// The name of the component.
	Name string

	// The settings it is using.
	Settings Map
}

// An Inspector can be used during development to print the applications
// component stack, the route table and log requests to writer.
type Inspector struct {
	Writer io.Writer
}

// DefaultInspector creates and returns a new inspector that writes to stdout.
func DefaultInspector() *Inspector {
	return NewInspector(os.Stdout)
}

// NewInspector creates and returns a new inspector.
func NewInspector(writer io.Writer) *Inspector {
	return &Inspector{
		Writer: writer,
	}
}

// Register implements the Component interface.
func (i *Inspector) Register(router *echo.Echo) {
	router.Use(i.requestLogger)
	router.SetHTTPErrorHandler(i.errorHandler)
}

// Inspect implements the InspectableComponent interface.
func (i *Inspector) Inspect() ComponentInfo {
	return ComponentInfo{
		Name: "Inspector",
	}
}

func (i *Inspector) boot() {
	fmt.Fprintln(i.Writer, "==> Fire application starting...")
}

func (i *Inspector) inspect(app *Application, router *echo.Echo) {
	// print component info
	fmt.Fprintln(i.Writer, "==> Mounted components:")
	i.inspectComponents(app)

	// print routing table
	fmt.Fprintln(i.Writer, "==> Registered routes:")
	i.inspectRoutingTable(router)
}

func (i *Inspector) run() {
	fmt.Fprintln(i.Writer, "==> Fire application is ready to go!")
}

func (i *Inspector) teardown() {
	fmt.Fprintln(i.Writer, "==> Fire application is stopping...")
}

func (i *Inspector) inspectComponents(app *Application) {
	// inspect all components
	for _, component := range app.components {
		// get component info
		info := component.Inspect()

		// print name
		fmt.Fprintf(i.Writer, "[%s]\n", info.Name)

		// prepare settings
		var settings []string

		// print settings
		for name, value := range info.Settings {
			settings = append(settings, fmt.Sprintf("  - %s: %s", name, value))
		}

		// sort settings
		sort.Strings(settings)

		// print settings
		for _, setting := range settings {
			fmt.Fprintln(i.Writer, setting)
		}
	}
}

func (i *Inspector) inspectRoutingTable(router *echo.Echo) {
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
		fmt.Fprintln(i.Writer, route)
	}
}

func (i *Inspector) requestLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		res := c.Response()

		// save start
		start := time.Now()

		// call next handler
		if err := next(c); err != nil {
			c.Error(err)
		}

		// get request duration
		duration := time.Since(start).String()

		// log request
		fmt.Fprintf(i.Writer, "%6s  %s\n   %d  %s\n", req.Method(), req.URL().Path(), res.Status(), duration)

		return nil
	}
}

func (i *Inspector) errorHandler(err error, ctx echo.Context) {
	fmt.Fprintf(i.Writer, "   ERR  \"%s\"\n", err)
}
