package flame

import (
	"net/http"

	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
)

// TokenMigrator is a middleware that detects access tokens passed via query
// parameters and migrates them to a "Bearer" token header. Additionally it may
// remove the migrated query parameter from the request.
//
// Note: The TokenMigrator should be added before any logger in the middleware
// chain to successfully protect the access token from being exposed.
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

// EnsureApplication will ensure that an application with the provided name
// exists and returns its key.
func EnsureApplication(store *coal.Store, name, key, secret string, redirectURIs ...string) (string, error) {
	// count main applications
	var apps []Application
	err := store.M(&Application{}).FindAll(nil, &apps, bson.M{
		"Name": name,
	}, nil, 0, 0, false, coal.NoTransaction)
	if err != nil {
		return "", err
	}

	// check existence
	if len(apps) > 1 {
		return "", xo.F("application name conflict")
	} else if len(apps) == 1 {
		return apps[0].Key, nil
	}

	/* application is missing */

	// create application
	app := &Application{
		Key:          key,
		Name:         name,
		Secret:       secret,
		RedirectURIs: redirectURIs,
	}

	// validate model
	err = app.Validate()
	if err != nil {
		return "", err
	}

	// save application
	err = store.M(app).Insert(nil, app)
	if err != nil {
		return "", err
	}

	return app.Key, nil
}

// EnsureFirstUser ensures the existence of a first user if no other has been
// created.
func EnsureFirstUser(store *coal.Store, name, email, password string) error {
	// check existence
	count, err := store.M(&User{}).Count(nil, bson.M{}, 0, 0, false, coal.NoTransaction)
	if err != nil {
		return err
	} else if count > 0 {
		return nil
	}

	/* user is missing */

	// create user
	user := &User{
		Name:     name,
		Email:    email,
		Password: password,
	}

	// set key and secret
	err = user.Validate()
	if err != nil {
		return err
	}

	// save user
	err = store.M(user).Insert(nil, user)
	if err != nil {
		return err
	}

	return nil
}
