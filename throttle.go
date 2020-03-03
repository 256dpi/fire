package fire

import "net/http"

// Throttle return a middleware that limits incoming to N parallel requests.
func Throttle(n int) func(http.Handler) http.Handler {
	// create bucket
	bucket := make(chan struct{}, n)

	// fill bucket
	for i := 0; i < n; i++ {
		bucket <- struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// get token
			select {
			case <-bucket:
			case <-r.Context().Done():
				return
			}

			// ensure token is added back
			defer func() {
				select {
				case bucket <- struct{}{}:
				default:
				}
			}()

			// call next
			next.ServeHTTP(w, r)
		})
	}
}
