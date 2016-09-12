package fire

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/labstack/echo"
)

// An Inspector can be used during development to print the route table and
// handle requests to writer.
//
// Note: Inspectors must be mounted as the last component in an application in
// oder to have access to the full routing table.
type Inspector struct {
	Writer io.Writer
}

// DefaultInspector creates and returns a new inspector that writes to stdout.
func DefaultInspector() *Inspector {
	return &Inspector{
		Writer: os.Stdout,
	}
}

// Register will register the inspector on the passed echo router.
func (i *Inspector) Register(router *echo.Echo) {
	i.inspectRoutingTable(router)
	router.Use(i.requestLogger)
}

func (i *Inspector) inspectRoutingTable(router *echo.Echo) {
	// print header
	fmt.Fprintln(i.Writer, "==> Fire application starting...")
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

	// print footer
	fmt.Fprintln(i.Writer, "==> Ready to go!")
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

		// fix path
		path := req.URL().Path()
		if path == "" {
			path = "/"
		}

		// log request
		fmt.Fprintf(i.Writer, "%6s  %-30s  %d  %s\n", req.Method(), path, res.Status(), duration)

		return nil
	}
}
