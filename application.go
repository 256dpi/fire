// Package fire implements a small and opinionated framework for Go providing
// Ember compatible JSON APIs.
package fire

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/pressly/chi"
	"github.com/tylerb/graceful"
	"gopkg.in/tomb.v2"
)

// Map is a general purpose map used for configuration.
type Map map[string]interface{}

// A Component that can be mounted in an application.
type Component interface {
	// Describe must return a ComponentInfo struct that describes the component.
	Describe() ComponentInfo
}

// A RoutableComponent is a component that accepts requests from a router for
// routes that haven been registered using Register().
type RoutableComponent interface {
	Component

	// Register will be called by the application with the chi router on
	// which the called components can register top level routes.
	Register(router chi.Router)
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

// A Phase is used in conjunction with a InspectorComponent and denotes a phase
// the application will undergo.
type Phase int

const (
	// Registration is the phase in which components get registered.
	Registration Phase = iota

	// Setup is the phase in which components get set up.
	Setup

	// Run is the phase in which the application handles requests.
	Run

	// Teardown is the phase in which components get teared down.
	Teardown

	// Termination is the phase in which the applications is terminated.
	Termination
)

// String returns the string representation of the phase.
func (p Phase) String() string {
	switch p {
	case Registration:
		return "Registration"
	case Setup:
		return "Setup"
	case Run:
		return "Run"
	case Teardown:
		return "Teardown"
	case Termination:
		return "Termination"
	}

	return ""
}

// An InspectorComponent is an extended component that is able to inspect the
// boot process of an application and inspect all used components and the router
// instance.
type InspectorComponent interface {
	Component

	// Before is called by the application before the specified phase is
	// initiated by the passed application.
	Before(phase Phase, app *Application)
}

// A ReporterComponent is an extended component that is responsible for
// reporting errors.
type ReporterComponent interface {
	Component

	// Report is called by the application on every occurring error.
	Report(err error) error
}

// An Application provides a simple way to combine multiple components.
//
// Note: An application instance is only valid for one cycle. This means it can
// only be started and stopped once.
type Application struct {
	components []Component
	routables  []RoutableComponent
	bootables  []BootableComponent
	inspectors []InspectorComponent
	reporters  []ReporterComponent

	router chi.Router

	server   *graceful.Server
	baseURL  string
	certFile string
	keyFile  string

	mutex sync.Mutex
	tomb  tomb.Tomb
}

// New creates and returns a new Application.
func New() *Application {
	return &Application{
		router: chi.NewRouter(),
	}
}

// Mount will mount the passed Component in the application.
//
// Note: Each component should only be mounted once before calling Run or Start.
func (a *Application) Mount(component Component) {
	// synchronize access
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// check status
	if a.server != nil {
		panic("Application has already been started")
	}

	// check component
	if component == nil {
		panic("Mount must be called with a component")
	}

	// add routable component
	if c, ok := component.(RoutableComponent); ok {
		a.routables = append(a.routables, c)
	}

	// add bootable component
	if c, ok := component.(BootableComponent); ok {
		a.bootables = append(a.bootables, c)
	}

	// add inspector
	if c, ok := component.(InspectorComponent); ok {
		a.inspectors = append(a.inspectors, c)
	}

	// add reporter
	if c, ok := component.(ReporterComponent); ok {
		a.reporters = append(a.reporters, c)
	}

	a.components = append(a.components, component)
}

// Components will return all so far registered components.
func (a *Application) Components() []Component {
	return a.components
}

// Router returns the used chi router for this application.
func (a *Application) Router() chi.Router {
	return a.router
}

// Start will start the application using a new server listening on the
// specified address.
//
// Note: Any errors that occur during the boot process of the application and
// later during request processing are reported using the registered reporters.
// If there are no reporters or one of the reporter fails to report the error,
// the calling goroutine will panic and print the error (see Exec).
func (a *Application) Start(addr string) {
	a.startWith("http://"+addr, &http.Server{Addr: addr}, "", "")
}

// StartSecure will start the application with a new server listening on the
// specified address using the provided TLS certificate.
//
// Note: Any errors that occur during the boot process of the application and
// later during request processing are reported using the registered reporters.
// If there are no reporters or one of the reporter fails to report the error,
// the calling goroutine will panic and print the error (see Exec).
func (a *Application) StartSecure(addr, certFile, keyFile string) {
	a.startWith("https://"+addr, &http.Server{Addr: addr}, certFile, keyFile)
}

// BaseURL returns the base URL of the application after it has ben started using
// Start or StartSecure.
func (a *Application) BaseURL() string {
	return a.baseURL
}

// Report will report the passed error using all mounted reporter components.
//
// Note: If a reporter fails to report an occurring error, the current goroutine
// will panic and print the original error and the reporter's error.
func (a *Application) Report(err error) {
	// prepare variable that tracks if the error has at least been reported once
	var reportedOnce bool

	// iterate over all reporters
	for _, r := range a.reporters {
		// attempt to report error
		rErr := r.Report(err)
		if rErr != nil {
			name := r.Describe().Name
			panic(fmt.Sprintf("%s returned '%s' while reporting '%s'", name, rErr, err))
		}

		// mark report
		reportedOnce = true
	}

	// check tracker
	if !reportedOnce {
		panic(fmt.Sprintf("No reporter found to report '%s'", err))
	}
}

// Exec will execute the passed function and report any potential errors.
//
// See: Report.
func (a *Application) Exec(fn func() error) {
	err := fn()
	if err != nil {
		a.Report(err)
	}
}

// Stop will stop a running application and wait until it has been properly stopped.
func (a *Application) Stop() {
	// synchronize access
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// kill controlling tomb
	a.tomb.Kill(nil)

	// stop app by stopping the server
	a.server.Stop(a.server.Timeout)

	// wait until goroutine finishes
	a.tomb.Wait()
}

// Yield will block the calling goroutine until the the application has been
// stopped. It will automatically stop the application if the process receives
// the SIGINT signal.
func (a *Application) Yield() {
	// prepare signal pipeline
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	select {
	// wait for interrupt and stop app
	case <-interrupt:
		a.Stop()
	// wait for app to close and return
	case <-a.tomb.Dead():
		return
	}
}

func (a *Application) startWith(baseURL string, server *http.Server, certFile, keyFile string) {
	// synchronize access
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// check status
	if a.server != nil {
		panic("Application has already been started")
	}

	// set handler on server
	server.Handler = a.router

	// create graceful server
	a.server = &graceful.Server{
		Timeout: 5 * time.Second,
		Server:  server,
	}

	// base url and tls files
	a.baseURL = baseURL
	a.certFile = certFile
	a.keyFile = keyFile

	// run app
	a.tomb.Go(a.runner)
}

func (a *Application) runner() error {
	a.Exec(a.boot)
	return nil
}

func (a *Application) boot() error {
	// signal before registration event
	for _, i := range a.inspectors {
		i.Before(Registration, a)
	}

	// register routable components
	for _, c := range a.routables {
		c.Register(a.router)
	}

	// signal before setup event
	for _, i := range a.inspectors {
		i.Before(Setup, a)
	}

	// setup bootable components
	for _, c := range a.bootables {
		err := c.Setup()
		if err != nil {
			return err
		}
	}

	// signal before run event
	for _, i := range a.inspectors {
		i.Before(Run, a)
	}

	// prepare error
	var err error

	// run server
	if a.certFile != "" && a.keyFile != "" {
		err = a.server.ListenAndServeTLS(a.certFile, a.keyFile)
	} else {
		err = a.server.ListenAndServe()
	}

	// handle error
	if err != nil {
		select {
		case <-a.tomb.Dying():
			// Stop() has been called and therefore the error returned by run
			// can be ignored as it is always the underlying listener failing
		default:
			return err
		}
	}

	// signal after run event
	for _, i := range a.inspectors {
		i.Before(Teardown, a)
	}

	// teardown bootable components
	for _, c := range a.bootables {
		err := c.Teardown()
		if err != nil {
			return err
		}
	}

	// signal after teardown event
	for _, i := range a.inspectors {
		i.Before(Termination, a)
	}

	return nil
}
