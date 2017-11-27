package flame

import (
	"errors"
	"net/http"

	"github.com/256dpi/fire/coal"
	"github.com/pborman/uuid"
	"gopkg.in/mgo.v2/bson"
)

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

// EnsureApplication will ensure that an application with the provided name
// exists.
func EnsureApplication(store *coal.Store, name string) error {
	// copy store
	s := store.Copy()
	defer s.Close()

	// count main applications
	var apps []Application
	err := s.C(&Application{}).Find(bson.M{
		coal.F(&Application{}, "Name"): name,
	}).All(&apps)
	if err != nil {
		return err
	}

	// check existence
	if len(apps) > 1 {
		return errors.New("to many applications with that name")
	} else if len(apps) == 1 {
		return nil
	}

	// application is missing

	// create application
	app := coal.Init(&Application{}).(*Application)
	app.Key = uuid.New()
	app.Name = name
	app.Secret = uuid.New()

	// validate model
	err = app.Validate()
	if err != nil {
		return err
	}

	// save application
	err = s.C(app).Insert(app)
	if err != nil {
		return err
	}

	return nil
}

// GetApplicationKey will return the key for the specified application.
func GetApplicationKey(store *coal.Store, name string) (string, error) {
	// copy store
	s := store.Copy()
	defer s.Close()

	// get application
	var app Application
	err := s.C(&app).Find(bson.M{
		"name": name,
	}).One(&app)
	if err != nil {
		return "", err
	}

	return app.Key, nil
}

// EnsureFirstUser ensures the existence of a first user if no other has been
// created.
func EnsureFirstUser(store *coal.Store, name, email, password string) error {
	// copy store
	s := store.Copy()
	defer s.Close()

	// check existence
	n, err := s.C(&User{}).Count()
	if err != nil {
		return err
	} else if n > 0 {
		return nil
	}

	// user is missing

	// create user
	user := coal.Init(&User{}).(*User)
	user.Name = name
	user.Email = email
	user.Password = password

	// set key and secret
	err = user.Validate()
	if err != nil {
		return err
	}

	// save user
	err = s.C(user).Insert(user)
	if err != nil {
		return err
	}

	return nil
}
