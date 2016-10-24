// Package components contains additional components which can be mounted in a
// fire application.
package components

//import (
//	"fmt"
//	"strings"
//
//	"github.com/gonfire/fire"
//	"github.com/labstack/echo"
//	"github.com/labstack/echo/middleware"
//	"github.com/pressly/chi"
//)
//
//var _ fire.RoutableComponent = (*Protector)(nil)
//
//// A Protector is a component that can be mounted in an application to enforce
//// common security concerns.
//type Protector struct {
//	// RequestBodyLimit defines the maximum size of a request body in the form
//	// of 4K, 2M, 1G or 1P.
//	RequestBodyLimit string
//
//	// AllowMethodOverriding will allow the usage of the X-HTTP-Method-Override
//	// header to set a request method when using the POST method.
//	AllowMethodOverriding bool
//
//	// AllowedCORSOrigins specifies the allowed origins when CORS.
//	AllowedCORSOrigins []string
//
//	// AllowedCORSHeaders specifies the allowed headers when CORS.
//	AllowedCORSHeaders []string
//
//	// AllowedCORSMethods specifies the allowed methods when CORS.
//	AllowedCORSMethods []string
//}
//
//// DefaultProtector returns a protector that is tailored to be used for JSON APIs.
//func DefaultProtector() *Protector {
//	return &Protector{
//		RequestBodyLimit:      "4K",
//		AllowMethodOverriding: false,
//		AllowedCORSOrigins: []string{
//			"*",
//		},
//		AllowedCORSHeaders: []string{
//			echo.HeaderOrigin,
//			echo.HeaderContentType,
//			echo.HeaderAuthorization,
//		},
//		AllowedCORSMethods: []string{
//			echo.GET,
//			echo.POST,
//			echo.PATCH,
//			echo.DELETE,
//		},
//	}
//}
//
//// Register implements the fire.RoutableComponent interface.
//func (p *Protector) Register(_ *fire.Application, router chi.Router) {
//	// set body limit
//	router.Use(middleware.BodyLimit(p.RequestBodyLimit))
//
//	// add gzip compression
//	router.Use(middleware.Gzip())
//
//	// enable method overriding
//	if p.AllowMethodOverriding {
//		router.Pre(middleware.MethodOverride())
//	}
//
//	// prepare allowed cors headers
//	allowedHeaders := p.AllowedCORSHeaders
//
//	// add method override header if enabled
//	if p.AllowMethodOverriding {
//		allowedHeaders = append(allowedHeaders, echo.HeaderXHTTPMethodOverride)
//	}
//
//	// add cors middleware
//	router.Use(middleware.CORSWithConfig(middleware.CORSConfig{
//		AllowOrigins: p.AllowedCORSOrigins,
//		AllowMethods: p.AllowedCORSMethods,
//		AllowHeaders: allowedHeaders,
//		MaxAge:       60,
//	}))
//
//	// prepare secure config
//	config := middleware.DefaultSecureConfig
//	config.XFrameOptions = ""
//
//	// TODO: Configure HSTS header.
//	// TODO: Force SSL by redirection.
//
//	// add the secure middleware
//	router.Use(middleware.SecureWithConfig(config))
//}
//
//// Describe implements the fire.Component interface.
//func (p *Protector) Describe() fire.ComponentInfo {
//	return fire.ComponentInfo{
//		Name: "Protector",
//		Settings: fire.Map{
//			"Request Body Limit":      p.RequestBodyLimit,
//			"Allow Method Overriding": fmt.Sprintf("%v", p.AllowMethodOverriding),
//			"Allowed CORS Origins":    strings.Join(p.AllowedCORSOrigins, ", "),
//			"Allowed CORS Methods":    strings.Join(p.AllowedCORSMethods, ", "),
//			"Allowed CORS Headers":    strings.Join(p.AllowedCORSHeaders, ", "),
//		},
//	}
//}
