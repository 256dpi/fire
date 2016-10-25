package fire

import (
	"net/http"
	"strings"
)

// The AssetServer component
type assetServer struct {
	prefix  string
	httpDir http.Dir
	httpFS  http.Handler
}

// DefaultAssetServer constructs an AssetServer that servers the directory on
// the root path.
func DefaultAssetServer(directory string) http.Handler {
	return NewAssetServer("", directory)
}

// NewAssetServer constructs an asset server handler that serves an asset
// directory on a specified path and serves the index file for not found paths
// which is needed to run single page applications like Ember.
func NewAssetServer(prefix, directory string) http.Handler {
	// ensure prefix
	prefix = "/" + strings.Trim(prefix, "/")

	// create dir server
	dir := http.Dir(directory)

	// create file server
	fs := http.FileServer(dir)

	return &assetServer{
		prefix:  prefix,
		httpDir: dir,
		httpFS:  fs,
	}
}

func (s *assetServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f, err := s.httpDir.Open(r.URL.Path)
	if err != nil {
		r.URL.Path = "/"
	} else if f != nil {
		f.Close()
	}

	s.httpFS.ServeHTTP(w, r)
}
