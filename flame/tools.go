package flame

import "net/http"

// TokenMigrator is a middleware that detects access tokens passed via query
// parameters and migrates them to a Bearer Token header. Additionally it may
// remove the migrated query parameter from the request.
//
// Note: The TokenMigrator should be added before any logger in the middleware
// chain to successfully protect the access_token from being exposed.
func TokenMigrator(remove bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// fetch access token
			accessToken := r.URL.Query().Get("access_token")

			// handle access token if present
			if accessToken != "" {
				// set token if not already set
				if r.Header.Get("Authorization") == "" {
					r.Header.Set("Authorization", "Bearer "+accessToken)
				}

				// remove parameter if requested
				if remove {
					q := r.URL.Query()
					q.Del("access_token")
					r.URL.RawQuery = q.Encode()
				}
			}

			// call next handler
			next.ServeHTTP(w, r)
		})
	}
}
