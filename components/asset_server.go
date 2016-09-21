package components

import (
	"fmt"

	"github.com/gonfire/fire"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var _ fire.RoutableComponent = (*AssetServer)(nil)

// The AssetServer component server an asset directory on a specified path and
// may optionally server the index file for not found paths which is needed to
// run single page applications like Ember.
type AssetServer struct {
	path      string
	directory string
	spaMode   bool
}

// DefaultAssetServer will create and return an AssetServer that is mounted on the
// root of the application with enabled SPA mode.
func DefaultAssetServer(directory string) *AssetServer {
	return NewAssetServer("", directory, true)
}

// NewAssetServer creates and returns a new AssetServer.
func NewAssetServer(path, directory string, spaMode bool) *AssetServer {
	return &AssetServer{
		path:      path,
		directory: directory,
		spaMode:   spaMode,
	}
}

// Register implements the fire.RoutableComponent interface.
func (s *AssetServer) Register(router *echo.Echo) {
	// create handler
	handler := middleware.StaticWithConfig(middleware.StaticConfig{
		Root:  s.directory,
		HTML5: s.spaMode,
	})

	// no path is set directly add handler to router
	if s.path == "" {
		router.Use(handler)
		return
	}

	// create group and add handler
	router.Group(s.path).Use(handler)
}

// Describe implements the fire.Component interface.
func (s *AssetServer) Describe() fire.ComponentInfo {
	// show root as slash
	path := s.path
	if path == "" {
		path = "/"
	}

	return fire.ComponentInfo{
		Name: "Asset Server",
		Settings: fire.Map{
			"Path":      path,
			"Directory": s.directory,
			"SPA Mode":  fmt.Sprintf("%v", s.spaMode),
		},
	}
}
