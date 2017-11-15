package wood

import (
	"net/http"
	"strings"
)

// DefaultAssetServer constructs an AssetServer that servers the directory on
// the root path.
func DefaultAssetServer(directory string) http.Handler {
	return NewAssetServer("/", directory)
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

	h := func(w http.ResponseWriter, r *http.Request) {
		// pre-check if file does exist
		f, err := dir.Open(r.URL.Path)
		if err != nil {
			r.URL.Path = "/"
		} else if f != nil {
			f.Close()
		}

		// serve file
		fs.ServeHTTP(w, r)
	}

	return http.StripPrefix(prefix, http.HandlerFunc(h))
}
