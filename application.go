package fire

import (
	"fmt"
	"sort"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/labstack/echo/middleware"
	"gopkg.in/mgo.v2"
)

// A Policy provides the security policy under which an applications is
// operating.
type Policy struct {
	// BodyLimit defines the maximum size of a request body in the form of 4K,
	// 2M, 1G or 1P.
	BodyLimit string

	// AllowMethodOverriding will allow the usage of the X-HTTP-Method-Override
	// header to set a request method when using the POST method.
	AllowMethodOverriding bool

	// AllowedCORSOrigins specifies the allowed origins when CORS.
	AllowedCORSOrigins []string

	// AllowedCORSHeaders specifies the allowed headers when CORS.
	AllowedCORSHeaders []string

	// AllowedCORSMethods specifies the allowed methods when CORS.
	AllowedCORSMethods []string

	// OverrideXFrameOptions can be set to override the default value
	// "SAMEORIGIN" for the "X-Frame-Option" header.
	OverrideXFrameOptions string

	// DisableAutomaticRecover will turn of automatic recover for panics.
	DisableAutomaticRecovery bool
}

// DefaultPolicy returns the default policy used by newly created applications.
func DefaultPolicy() Policy {
	return Policy{
		BodyLimit:             "4K",
		AllowMethodOverriding: false,
		AllowedCORSOrigins: []string{
			"*",
		},
		AllowedCORSHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAuthorization,
		},
		AllowedCORSMethods: []string{
			echo.GET,
			echo.POST,
			echo.PATCH,
			echo.DELETE,
		},
	}
}

// A Component can be mounted on an application.
type Component interface {
	Register(router *echo.Echo)
}

// An Application provides an out-of-the-box configuration of components to
// get started with building JSON APIs.
type Application struct {
	components []Component
	policy     Policy
	router     *echo.Echo
	session    *mgo.Session

	enableDevMode bool
}

// New creates and returns a new Application.
func New(mongoURI, prefix string) *Application {
	// create router
	router := echo.New()

	// connect to database
	sess, err := mgo.Dial(mongoURI)
	if err != nil {
		panic(err)
	}

	return &Application{
		policy:  DefaultPolicy(),
		router:  router,
		session: sess,
	}
}

// SetPolicy will set a new policy.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) SetPolicy(policy Policy) {
	a.policy = policy
}

// Mount will mount the passed Component in the application using the passed
// prefix.
//
// Note: Each component should only be mounted once before calling Run or Start.
func (a *Application) Mount(component Component) {
	component.Register(a.router)
}

// CloneSession will return a freshly cloned session.
//
// Note: You need to close the session when finished.
func (a *Application) CloneSession() *mgo.Session {
	return a.session.Clone()
}

// EnableDevMode will enable the development mode that prints all registered
// handlers on boot and all incoming requests.
func (a *Application) EnableDevMode() {
	a.enableDevMode = true
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) {
	a.prepare()
	a.printDevInfo()
	a.router.Run(standard.New(addr))
}

// SecureStart will run the application on the specified address using a TLS
// certificate.
func (a *Application) SecureStart(addr, certFile, keyFile string) {
	a.prepare()
	a.printDevInfo()
	a.router.Run(standard.WithTLS(addr, certFile, keyFile))
}

func (a *Application) prepare() {
	// set body limit
	a.router.Use(middleware.BodyLimit(a.policy.BodyLimit))

	// add gzip compression
	a.router.Use(middleware.Gzip())

	// enable method overriding
	if a.policy.AllowMethodOverriding {
		a.router.Pre(middleware.MethodOverride())
	}

	// prepare allowed cors headers
	allowedHeaders := a.policy.AllowedCORSHeaders

	// add method override header if enabled
	if a.policy.AllowMethodOverriding {
		allowedHeaders = append(allowedHeaders, echo.HeaderXHTTPMethodOverride)
	}

	// add cors middleware
	a.router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: a.policy.AllowedCORSOrigins,
		AllowMethods: a.policy.AllowedCORSMethods,
		AllowHeaders: allowedHeaders,
		MaxAge:       60,
	}))

	// enable automatic recovery
	if !a.policy.DisableAutomaticRecovery {
		a.router.Use(middleware.Recover())
	}

	// prepare secure config
	config := middleware.DefaultSecureConfig

	// override X-Frame-Options if available
	if len(a.policy.OverrideXFrameOptions) > 0 {
		config.XFrameOptions = a.policy.OverrideXFrameOptions
	}

	// TODO: Configure HSTS header.

	// TODO: Force SSL by redirection.

	// add the secure middleware
	a.router.Use(middleware.SecureWithConfig(config))

	// enable dev mode
	if a.enableDevMode {
		a.router.Use(a.logger)
	}
}

func (a *Application) printDevInfo() {
	// return if not enabled
	if !a.enableDevMode {
		return
	}

	// print header
	fmt.Println("==> Fire application starting...")
	fmt.Println("==> Registered routes:")

	// prepare routes
	var routes []string

	// add all routes as string
	for _, route := range a.router.Routes() {
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
