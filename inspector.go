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

var _ InspectorComponent = (*Inspector)(nil)

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

// Describe implements the Component interface.
func (i *Inspector) Describe() ComponentInfo {
	return ComponentInfo{
		Name: "Inspector",
	}
}

// Register implements the RoutableComponent interface.
func (i *Inspector) Register(router *echo.Echo) {
	router.Use(i.requestLogger)
}

// BeforeRegister implements the InspectorComponent interface.
func (i *Inspector) BeforeRegister(components []Component) {
	fmt.Fprintln(i.Writer, "==> Application starting...")
	fmt.Fprintln(i.Writer, "==> Registering routable components...")

	// inspect all components
	for _, component := range components {
		// get component info
		info := component.Describe()

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

// BeforeSetup implements the InspectorComponent interface.
func (i *Inspector) BeforeSetup(components []BootableComponent) {
	fmt.Fprintln(i.Writer, "==> Setting up bootable components...")

	// print all components
	for _, component := range components {
		// get component info
		fmt.Fprintf(i.Writer, "%s\n", component.Describe().Name)
	}
}

// BeforeRun implements the InspectorComponent interface.
func (i *Inspector) BeforeRun(router *echo.Echo) {
	fmt.Fprintln(i.Writer, "==> Registered routes:")

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

	fmt.Fprintln(i.Writer, "==> Application is ready to go!")
}

// AfterRun implements the InspectorComponent interface.
func (i *Inspector) AfterRun() {
	fmt.Fprintln(i.Writer, "==> Application is stopping...")
}

// AfterTeardown implements the InspectorComponent interface.
func (i *Inspector) AfterTeardown() {
	fmt.Fprintln(i.Writer, "==> Application has been terminate.")
}

// Report implements the ReporterComponent interface.
func (i *Inspector) Report(err error) error {
	fmt.Fprintf(i.Writer, "   ERR  \"%s\"\n", err)
	return nil
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
