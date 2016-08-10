package fire

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/appleboy/gofight"
	"github.com/stretchr/testify/assert"
)

func TestPasswordGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), &User{}, &Application{}, secret, "fire")
	authenticator.EnablePasswordGrant()

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer(),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:   "Test Application",
		Key:    "key1",
		Secret: authenticator.MustHashPassword("secret"),
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user1@example.com",
		Password: authenticator.MustHashPassword("secret"),
	})

	r := gofight.New()

	// missing auth
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong secret
	r.POST("/auth/token").
		SetHeader(basicAuth("key1", "secret")).
		SetFORM(gofight.H{
			"grant_type": "password",
			"username":   "user1@example.com",
			"password":   "wrong-secret",
			"scope":      "fire",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong scope
	r.POST("/auth/token").
		SetHeader(basicAuth("key1", "secret")).
		SetFORM(gofight.H{
			"grant_type": "password",
			"username":   "user1@example.com",
			"password":   "secret",
			"scope":      "wrong",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key1", "secret")).
		SetFORM(gofight.H{
			"grant_type": "password",
			"username":   "user1@example.com",
			"password":   "secret",
			"scope":      "fire",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "fire", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}

func TestCredentialsGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), &User{}, &Application{}, secret, "fire")
	authenticator.EnableCredentialsGrant()

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer(),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:   "Test Application",
		Key:    "key2",
		Secret: authenticator.MustHashPassword("secret"),
	})

	r := gofight.New()

	// missing auth
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong secret
	r.POST("/auth/token").
		SetHeader(basicAuth("key2", "wrong-secret")).
		SetFORM(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "fire",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong scope
	r.POST("/auth/token").
		SetHeader(basicAuth("key2", "secret")).
		SetFORM(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "wrong",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key2", "secret")).
		SetFORM(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "fire",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "fire", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}

func TestImplicitGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), &User{}, &Application{}, secret, "fire")
	authenticator.EnableImplicitGrant()

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer(),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:     "Test Application",
		Key:      "key3",
		Secret:   authenticator.MustHashPassword("secret"),
		Callback: "https://0.0.0.0:8080/auth/callback",
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user2@example.com",
		Password: authenticator.MustHashPassword("secret"),
	})

	r := gofight.New()

	// missing auth
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong secret
	r.POST("/auth/authorize").
		SetFORM(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "fire",
			"username":      "user2@example.com",
			"password":      "wrong-secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.NotEmpty(t, loc.RawQuery)
		})

	// wrong scope
	r.POST("/auth/authorize").
		SetFORM(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "wrong",
			"username":      "user2@example.com",
			"password":      "secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.NotEmpty(t, loc.RawQuery)
		})

	var token string

	// get access token
	r.POST("/auth/authorize").
		SetFORM(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "fire",
			"username":      "user2@example.com",
			"password":      "secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			query, err := url.ParseQuery(loc.Fragment)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.Equal(t, "3600", query.Get("expires_in"))
			assert.Equal(t, "fire", query.Get("scope"))
			assert.Equal(t, "bearer", query.Get("token_type"))

			token = query.Get("access_token")
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}
