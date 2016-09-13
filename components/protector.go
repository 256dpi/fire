// Package components contains additional components which can be mounted in a
// fire application.
package components

import (
	"fmt"
	"strings"

	"github.com/gonfire/fire"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

// A Protector is a component that can be mounted in an application to enforce
// common security concerns.
type Protector struct {
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

	// XFrameOptions will set the "X-Frame-Option" header.
	XFrameOptions string

	// DisableAutomaticRecover will turn of automatic recover for panics.
	DisableAutomaticRecovery bool
}

// DefaultProtector returns a protector that is tailored to be used for JSON APIs.
func DefaultProtector() *Protector {
	return &Protector{
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
		XFrameOptions: "SAMEORIGIN",
	}
}

// Register will register the protector on the passed echo router.
func (p *Protector) Register(router *echo.Echo) {
	// set body limit
	router.Use(middleware.BodyLimit(p.BodyLimit))

	// add gzip compression
	router.Use(middleware.Gzip())

	// enable method overriding
	if p.AllowMethodOverriding {
		router.Pre(middleware.MethodOverride())
	}

	// prepare allowed cors headers
	allowedHeaders := p.AllowedCORSHeaders

	// add method override header if enabled
	if p.AllowMethodOverriding {
		allowedHeaders = append(allowedHeaders, echo.HeaderXHTTPMethodOverride)
	}

	// add cors middleware
	router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: p.AllowedCORSOrigins,
		AllowMethods: p.AllowedCORSMethods,
		AllowHeaders: allowedHeaders,
		MaxAge:       60,
	}))

	// enable automatic recovery
	if !p.DisableAutomaticRecovery {
		router.Use(middleware.Recover())
	}

	// prepare secure config
	config := middleware.DefaultSecureConfig

	// override X-Frame-Options if available
	if len(p.XFrameOptions) > 0 {
		config.XFrameOptions = p.XFrameOptions
	}

	// TODO: Configure HSTS header.
	// TODO: Force SSL by redirection.

	// add the secure middleware
	router.Use(middleware.SecureWithConfig(config))
}

// Inspect implements the InspectableComponent interface.
func (p *Protector) Inspect() fire.ComponentInfo {
	return fire.ComponentInfo{
		Name: "Protector",
		Settings: fire.Map{
			"Body Limit":              p.BodyLimit,
			"Allow Method Overriding": fmt.Sprintf("%v", p.AllowMethodOverriding),
			"Allowed CORS Origins":    strings.Join(p.AllowedCORSOrigins, ", "),
			"Allowed CORS Methods":    strings.Join(p.AllowedCORSMethods, ", "),
			"Allowed CORS Headers":    strings.Join(p.AllowedCORSHeaders, ", "),
			"Automatic Recovery":      fmt.Sprintf("%v", !p.DisableAutomaticRecovery),
			"X-Frame-Options":         p.XFrameOptions,
		},
	}
}
