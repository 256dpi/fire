package fire

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/appleboy/gofight"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

func init() {
	// override default hash cost for token generation to speedup tests
	hashCost = bcrypt.MinCost
}

func TestEnableOnlyOnce(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnablePasswordGrant()
	authenticator.EnableCredentialsGrant()
	authenticator.EnableImplicitGrant()

	assert.Panics(t, func() {
		authenticator.EnablePasswordGrant()
	})

	assert.Panics(t, func() {
		authenticator.EnableCredentialsGrant()
	})

	assert.Panics(t, func() {
		authenticator.EnableImplicitGrant()
	})
}

func TestPasswordGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnablePasswordGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "password", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer(&Resource{
		Model: &Post{},
		Authorizer: Combine(
			authenticator.Authorizer("default"),
			func(ctx *Context) error {
				assert.NotNil(t, ctx.GinContext.MustGet("fire.access_token"))
				return nil
			},
		),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key1",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{"password"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user1@example.com",
		Password: hashPassword("secret"),
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
		SetForm(gofight.H{
			"grant_type": "password",
			"username":   "user1@example.com",
			"password":   "wrong-secret",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong scope
	r.POST("/auth/token").
		SetHeader(basicAuth("key1", "secret")).
		SetForm(gofight.H{
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
		SetForm(gofight.H{
			"grant_type": "password",
			"username":   "user1@example.com",
			"password":   "secret",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default", json.Path("scope").Data().(string))
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

	// check issued access token
	accessToken := &AccessToken{}
	findModel(db, accessToken, bson.M{
		"signature": strings.Split(token, ".")[1],
	})
	assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
	assert.True(t, accessToken.ClientID.Valid())
	assert.True(t, accessToken.OwnerID.Valid())
}

func TestCredentialsGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnableCredentialsGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "client_credentials", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer(&Resource{
		Model: &Post{},
		Authorizer: Combine(
			authenticator.Authorizer("default"),
			func(ctx *Context) error {
				assert.NotNil(t, ctx.GinContext.MustGet("fire.access_token"))
				return nil
			},
		),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key2",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{"client_credentials"},
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
		SetForm(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})

	// wrong scope
	r.POST("/auth/token").
		SetHeader(basicAuth("key2", "secret")).
		SetForm(gofight.H{
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
		SetForm(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default", json.Path("scope").Data().(string))
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

	// check issued access token
	accessToken := &AccessToken{}
	findModel(db, accessToken, bson.M{
		"signature": strings.Split(token, ".")[1],
	})
	assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
	assert.True(t, accessToken.ClientID.Valid())
	assert.Nil(t, accessToken.OwnerID)
}

func TestImplicitGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnableImplicitGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "implicit", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer(&Resource{
		Model: &Post{},
		Authorizer: Combine(
			authenticator.Authorizer("default"),
			func(ctx *Context) error {
				assert.NotNil(t, ctx.GinContext.MustGet("fire.access_token"))
				return nil
			},
		),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key3",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{"implicit"},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user3@example.com",
		Password: hashPassword("secret"),
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
		SetForm(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "default",
			"username":      "user3@example.com",
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
		SetForm(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "wrong",
			"username":      "user3@example.com",
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
		SetForm(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "default",
			"username":      "user3@example.com",
			"password":      "secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			query, err := url.ParseQuery(loc.Fragment)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.Equal(t, "3600", query.Get("expires_in"))
			assert.Equal(t, "default", query.Get("scope"))
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

	// check issued access token
	accessToken := &AccessToken{}
	findModel(db, accessToken, bson.M{
		"signature": strings.Split(token, ".")[1],
	})
	assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
	assert.True(t, accessToken.ClientID.Valid())
	assert.True(t, accessToken.OwnerID.Valid())
}

func TestPasswordGrantAdditionalScope(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnablePasswordGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "password", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return []string{"default", "admin"}
	}

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key4",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{"password"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user4@example.com",
		Password: hashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key4", "secret")).
		SetForm(gofight.H{
			"grant_type": "password",
			"username":   "user4@example.com",
			"password":   "secret",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default admin", json.Path("scope").Data().(string))
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

func TestPasswordGrantInsufficientScope(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnablePasswordGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "password", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key5",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{"password"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user5@example.com",
		Password: hashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key5", "secret")).
		SetForm(gofight.H{
			"grant_type": "password",
			"username":   "user5@example.com",
			"password":   "secret",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// failing to get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})
}

func TestCredentialsGrantAdditionalScope(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnableCredentialsGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "client_credentials", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return []string{"default", "admin"}
	}

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key6",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{"client_credentials"},
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key6", "secret")).
		SetForm(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default admin", json.Path("scope").Data().(string))
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

func TestCredentialsGrantInsufficientScope(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnableCredentialsGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "client_credentials", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key7",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{"client_credentials"},
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key7", "secret")).
		SetForm(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// failing to get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})
}

func TestImplicitGrantAdditionalScope(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnableImplicitGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "implicit", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return []string{"default", "admin"}
	}

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key8",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{"implicit"},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user8@example.com",
		Password: hashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/authorize").
		SetForm(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key8",
			"state":         "state1234",
			"scope":         "default",
			"username":      "user8@example.com",
			"password":      "secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			query, err := url.ParseQuery(loc.Fragment)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.Equal(t, "3600", query.Get("expires_in"))
			assert.Equal(t, "default+admin", query.Get("scope"))
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

func TestImplicitGrantInsufficientScope(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnableImplicitGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "implicit", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key9",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{"implicit"},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user9@example.com",
		Password: hashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/authorize").
		SetForm(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key9",
			"state":         "state1234",
			"scope":         "default",
			"username":      "user9@example.com",
			"password":      "secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			query, err := url.ParseQuery(loc.Fragment)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.Equal(t, "3600", query.Get("expires_in"))
			assert.Equal(t, "default", query.Get("scope"))
			assert.Equal(t, "bearer", query.Get("token_type"))

			token = query.Get("access_token")
		})

	// failing to get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.NotEmpty(t, r.Body.String())
		})
}

func TestGinAuthorizer(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), secret, 0)
	authenticator.SetModels(&User{}, &Application{}, &AccessToken{})
	authenticator.EnablePasswordGrant()

	authenticator.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, "password", req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	server, db := buildServer()
	server.GET("/foo", authenticator.GinAuthorizer("default"), func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "OK")
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key10",
		Secret:     hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{"password"},
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user10@example.com",
		Password: hashPassword("secret"),
	})

	r := gofight.New()

	// missing auth
	r.GET("/foo").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusUnauthorized, r.Code)
			assert.Empty(t, r.Body.String())
		})

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key10", "secret")).
		SetForm(gofight.H{
			"grant_type": "password",
			"username":   "user10@example.com",
			"password":   "secret",
			"scope":      "default",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "default", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// get empty list of posts
	r.GET("/foo").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "OK", r.Body.String())
		})

	// check issued access token
	accessToken := &AccessToken{}
	findModel(db, accessToken, bson.M{
		"signature": strings.Split(token, ".")[1],
	})
	assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
	assert.True(t, accessToken.ClientID.Valid())
	assert.True(t, accessToken.OwnerID.Valid())
}
