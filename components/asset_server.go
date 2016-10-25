package components

import (
	"net/http"
	"strings"
)

// The AssetServer component server an asset directory on a specified path and
// may optionally server the index file for not found paths which is needed to
// run single page applications like Ember.
type AssetServer struct {
	prefix  string
	httpDir http.Dir
	httpFS  http.Handler
}

// DefaultAssetServer will create and return an AssetServer that is mounted on the
// root of the application with enabled SPA mode.
func DefaultAssetServer(directory string) *AssetServer {
	return NewAssetServer("", directory)
}

// NewAssetServer creates and returns a new AssetServer.
func NewAssetServer(prefix, directory string) *AssetServer {
	// ensure prefix
	prefix = "/" + strings.Trim(prefix, "/")

	// create dir server
	dir := http.Dir(directory)

	// create file server
	fs := http.FileServer(dir)

	return &AssetServer{
		prefix:  prefix,
		httpDir: dir,
		httpFS:  fs,
	}
}

func (s *AssetServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, err := s.httpDir.Open(r.URL.Path)
	if err != nil {
		r.URL.Path = "/"
	} else if f != nil {
		f.Close()
	}

	s.httpFS.ServeHTTP(w, r)
}
