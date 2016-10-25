package fire

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
	"os"

	"github.com/pressly/chi"
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
func (i *Inspector) Register(_ *Application, router chi.Router) {
	router.Use(i.requestLogger)
}

// Before implements the InspectorComponent interface.
func (i *Inspector) Before(app *Application, phase Phase) {
	switch phase {
	case Registration:
		fmt.Fprintln(i.Writer, "==> Application booting...")

		fmt.Fprintln(i.Writer, "==> Mounted components:")
		i.printComponents(app.Components())

		fmt.Fprintln(i.Writer, "==> Registering routable components...")
	case Setup:
		fmt.Fprintln(i.Writer, "==> Setting up bootable components...")
	case Run:
		fmt.Fprintln(i.Writer, "==> Registered routes:")
		i.printRoutes(app.Router())

		fmt.Fprintln(i.Writer, "==> Application is ready to go!")
		fmt.Fprintln(i.Writer, "==> Visit: %s", app.BaseURL())
	case Teardown:
		fmt.Fprintln(i.Writer, "==> Application is stopping...")
		fmt.Fprintln(i.Writer, "==> Terminating bootable components...")
	case Termination:
		fmt.Fprintln(i.Writer, "==> Application has been terminated.")
	}
}

// Report implements the ReporterComponent interface.
func (i *Inspector) Report(_ *Application, err error) error {
	fmt.Fprintf(i.Writer, "   ERR  \"%s\"\n", err)
	return nil
}

func (i *Inspector) printComponents(components []Component) {
	// inspect all components
	for _, component := range components {
		// get component info
		info := component.Describe()

		// print name
		fmt.Fprintln(i.Writer, "[%s]", info.Name)

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

func (i *Inspector) printRoutes(router chi.Router) {
	// prepare routes
	var routes []string

	// add all routes as string
	for _, route := range router.Routes() {
		for method := range route.Handlers {
			routes = append(routes, fmt.Sprintf("%6s  %-30s", method, route.Pattern))
		}
	}

	// sort routes
	sort.Strings(routes)

	// print routes
	for _, route := range routes {
		fmt.Fprintln(i.Writer, route)
	}
}

func (i *Inspector) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// wrap response writer
		wrw := wrapResponseWriter(w)

		// save start
		start := time.Now()

		// call next handler
		next.ServeHTTP(wrw, r)

		// get request duration
		duration := time.Since(start).String()

		// log request
		fmt.Fprintf(i.Writer, "%6s  %s\n   %s  %s\n", r.Method, r.URL.Path, wrw.Status(), duration)
	})
}

type wrappedResponseWriter struct {
	status int
	http.ResponseWriter
}

func wrapResponseWriter(res http.ResponseWriter) *wrappedResponseWriter {
	// default the status code to 200
	return &wrappedResponseWriter{200, res}
}

func (w wrappedResponseWriter) Status() int {
	return w.status
}

func (w wrappedResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func (w wrappedResponseWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func (w wrappedResponseWriter) WriteHeader(statusCode int) {
	// Store the status code
	w.status = statusCode

	// Write the status code onward.
	w.ResponseWriter.WriteHeader(statusCode)
}
