package components

import (
	"fmt"
	"net/http"

	"github.com/gonfire/fire"
	"github.com/pressly/chi"
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
func (s *AssetServer) Register(_ *fire.Application, router chi.Router) {
	// prepare prefixed path
	prefixedPath := s.path

	// check empty string and non leading string
	if s.path == "" {
		prefixedPath = "/"
	} else if s.path[0] != '/' {
		prefixedPath = "/" + s.path
	}

	// create dir server
	dir := http.Dir(s.directory)

	// create file server
	fs := http.FileServer(dir)

	// prepare file handler
	srv := fs

	// add a in between handler that checks for missing files and returns
	// the directory index
	if s.spaMode {
		srv = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f, err := dir.Open(r.URL.Path)
			if err != nil {
				r.URL.Path = "/"
			} else if f != nil {
				f.Close()
			}

			fs.ServeHTTP(w, r)
		})
	}

	router.Get(prefixedPath+"*", http.StripPrefix(s.path, srv).ServeHTTP)
}

// Describe implements the fire.Component interface.
func (s *AssetServer) Describe() fire.ComponentInfo {
	return fire.ComponentInfo{
		Name: "Asset Server",
		Settings: fire.Map{
			"Path":      s.path,
			"Directory": s.directory,
			"SPA Mode":  fmt.Sprintf("%v", s.spaMode),
		},
	}
}
