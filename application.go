package fire

import (
	"fmt"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"github.com/labstack/echo/middleware"
	"gopkg.in/mgo.v2"
)

// An Application provides an out-of-the-box configuration of components to
// get started with building JSON APIs.
type Application struct {
	set    *Set
	router *echo.Echo

	bodyLimit      string
	allowedOrigins []string
	allowedHeaders []string

	forceEncryption        bool
	disableCORS            bool
	disableCompression     bool
	disableRecovery        bool
	disableCommonSecurity  bool
	enableMethodOverriding bool
	enableDevMode          bool
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

	// create controller set
	set := NewSet(sess, router, prefix)

	return &Application{
		set:            set,
		router:         router,
		bodyLimit:      "4K",
		allowedOrigins: []string{"*"},
		allowedHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAuthorization,
		},
	}
}

// Mount will add controllers to the set and register them on the router.
//
// Note: Each controller should only be mounted once.
func (a *Application) Mount(controllers ...*Controller) {
	a.set.Mount(controllers...)
}

// Router will return the internally used echo instance.
func (a *Application) Router() *echo.Echo {
	return a.router
}

// ForceEncryption will make the application enforce and only respond to
// encrypted requests.
//
// Note: ForceEncryption will be automatically enabled when calling SecureStart.
func (a *Application) ForceEncryption() {
	a.forceEncryption = true
}

// EnableMethodOverriding will enable the usage of the X-HTTP-Method-Override
// header to set a request method when using the POST method.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) EnableMethodOverriding() {
	a.enableMethodOverriding = true
}

// SetBodyLimit can be used to override the default body limit of 4K with a new
// value in the form of 4K, 2M, 1G or 1P.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) SetBodyLimit(size string) {
	a.bodyLimit = size
}

// SetAllowedOrigins will replace the default allowed origin set `*`.
func (a *Application) SetAllowedOrigins(origins ...string) {
	a.allowedOrigins = origins
}

// AddAllowedHeaders will allow additional headers.
func (a *Application) AddAllowedHeaders(headers ...string) {
	a.allowedHeaders = append(a.allowedHeaders, headers...)
}

// DisableCORS will turn off CORS support.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) DisableCORS(origins ...string) {
	a.disableCORS = true
}

// DisableCompression will turn of gzip compression.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) DisableCompression() {
	a.disableCompression = true
}

// DisableRecovery will disable the automatic recover mechanism.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) DisableRecovery() {
	a.disableRecovery = true
}

// DisableCommonSecurity will disable common security features including:
// protection against cross-site scripting attacks by setting the
// `X-XSS-Protection` header, protection against overriding Content-Type
// header by setting the `X-Content-Type-Options` header and protection against
// clickjacking by setting the `X-Frame-Options` header.
//
// Note: This method must be called before calling Run or Start.
func (a *Application) DisableCommonSecurity() {
	a.disableCommonSecurity = true
}

// EnableDevMode will enable the development mode that prints all registered
// handlers on boot and all incoming requests.
func (a *Application) EnableDevMode() {
	a.enableDevMode = true
}

// Start will run the application on the specified address.
func (a *Application) Start(addr string) {
	a.run(standard.New(addr))
}

// SecureStart will run the application on the specified address using a TLS
// certificate.
func (a *Application) SecureStart(addr, certFile, keyFile string) {
	a.forceEncryption = true

	a.run(standard.WithTLS(addr, certFile, keyFile))
}

func (a *Application) run(server engine.Server) {
	// set body limit
	a.router.Use(middleware.BodyLimit(a.bodyLimit))

	// force encryption
	if a.forceEncryption {
		// TODO: register https redirect middleware with next release
		// a.router.Pre(middleware.HTTPSRedirect())
	}

	// enable cors
	if !a.disableCORS {
		allowedHeaders := a.allowedHeaders

		// add method override header if enabled
		if a.enableMethodOverriding {
			allowedHeaders = append(allowedHeaders, echo.HeaderXHTTPMethodOverride)
		}

		// add cors middleware
		a.router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins: a.allowedOrigins,
			AllowMethods: []string{echo.GET, echo.POST, echo.PATCH, echo.DELETE},
			AllowHeaders: allowedHeaders,
			MaxAge:       60,
		}))
	}

	// enable gzip compression
	if !a.disableCompression {
		a.router.Use(middleware.Gzip())
	}

	// enable automatic recovery
	if !a.disableRecovery {
		a.router.Use(middleware.Recover())
	}

	// enable common security
	if !a.disableCommonSecurity {
		config := middleware.DefaultSecureConfig

		// keep using TLS for 60 minutes on just that domain
		// TODO: Make that configurable.
		if a.forceEncryption {
			config.HSTSMaxAge = 3600
			config.HSTSExcludeSubdomains = true
		}

		a.router.Use(middleware.SecureWithConfig(config))
	}

	// enable method overriding
	if a.enableMethodOverriding {
		a.router.Pre(middleware.MethodOverride())
	}

	// enable dev mode
	if a.enableDevMode {
		a.printInfo()
		a.router.Use(a.logger)
	}

	a.router.Run(server)
}

func (a *Application) printInfo() {
	fmt.Println("==> Fire application starting...")
	fmt.Println("==> Registered routes:")

	// TODO: Order routes.

	for _, route := range a.router.Routes() {
		fmt.Printf("%6s  %-30s\n", route.Method, route.Path)
	}

	fmt.Println("==> Ready to go!")
}

func (a *Application) logger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		req := c.Request()
		res := c.Response()

		start := time.Now()
		if err = next(c); err != nil {
			c.Error(err)
		}

		duration := time.Since(start).String()

		path := req.URL().Path()
		if path == "" {
			path = "/"
		}

		fmt.Printf("%6s  %-30s  %d  %s\n", req.Method(), path, res.Status(), duration)

		return
	}
}
