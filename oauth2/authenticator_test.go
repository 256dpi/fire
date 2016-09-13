package oauth2

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/gonfire/fire/jsonapi"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

var policy *Policy

func init() {
	// override default hash cost for token generation to speedup tests
	hashCost = bcrypt.MinCost

	// setup policy
	policy = DefaultPolicy()
	policy.Secret = []byte("a-very-long-secret")
}

func TestPasswordGrant(t *testing.T) {
	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model: &Post{},
		Pool:  getPool(),
		Authorizer: jsonapi.Combine(
			authenticator.Authorizer("default"),
			func(ctx *jsonapi.Context) error {
				assert.NotNil(t, ctx.Echo.Get("fire.access_token"))
				return nil
			},
		),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key1",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user1@example.com",
		PasswordHash: hashPassword("secret"),
	})

	// missing auth
	testRequest(server, "GET", "/posts", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	// wrong secret
	testRequest(server, "POST", "/auth/token", basicAuth("key1", "secret"), M{
		"grant_type": PasswordGrant,
		"username":   "user1@example.com",
		"password":   "wrong-secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	// wrong scope
	testRequest(server, "POST", "/auth/token", basicAuth("key1", "secret"), M{
		"grant_type": PasswordGrant,
		"username":   "user1@example.com",
		"password":   "secret",
		"scope":      "wrong",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key1", "secret"), M{
		"grant_type": PasswordGrant,
		"username":   "user1@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
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

func TestClientCredentialsGrant(t *testing.T) {
	policy.ClientCredentialsGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ClientCredentialsGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model: &Post{},
		Pool:  getPool(),
		Authorizer: jsonapi.Combine(
			authenticator.Authorizer("default"),
			func(ctx *jsonapi.Context) error {
				assert.NotNil(t, ctx.Echo.Get("fire.access_token"))
				return nil
			},
		),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key2",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{ClientCredentialsGrant},
	})

	// missing auth
	testRequest(server, "GET", "/posts", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	// wrong secret
	testRequest(server, "POST", "/auth/token", basicAuth("key2", "wrong-secret"), M{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	// wrong scope
	testRequest(server, "POST", "/auth/token", basicAuth("key2", "secret"), M{
		"grant_type": ClientCredentialsGrant,
		"scope":      "wrong",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key2", "secret"), M{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
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
	policy.ImplicitGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ImplicitGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model: &Post{},
		Pool:  getPool(),
		Authorizer: jsonapi.Combine(
			authenticator.Authorizer("default"),
			func(ctx *jsonapi.Context) error {
				assert.NotNil(t, ctx.Echo.Get("fire.access_token"))
				return nil
			},
		),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key3",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{ImplicitGrant},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user3@example.com",
		PasswordHash: hashPassword("secret"),
	})

	// missing auth
	testRequest(server, "GET", "/posts", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})

	// wrong secret
	testRequest(server, "POST", "/auth/authorize", nil, M{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key3",
		"state":         "state1234",
		"scope":         "default",
		"username":      "user3@example.com",
		"password":      "wrong-secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		loc, err := url.Parse(r.Header().Get("Location"))
		assert.NoError(t, err)

		assert.Equal(t, http.StatusFound, r.Code)
		assert.NotEmpty(t, loc.RawQuery)
	})

	// wrong scope
	testRequest(server, "POST", "/auth/authorize", nil, M{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key3",
		"state":         "state1234",
		"scope":         "wrong",
		"username":      "user3@example.com",
		"password":      "secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		loc, err := url.Parse(r.Header().Get("Location"))
		assert.NoError(t, err)

		assert.Equal(t, http.StatusFound, r.Code)
		assert.NotEmpty(t, loc.RawQuery)
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/authorize", nil, M{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key3",
		"state":         "state1234",
		"scope":         "default",
		"username":      "user3@example.com",
		"password":      "secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		loc, err := url.Parse(r.Header().Get("Location"))
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
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
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
	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return []string{"default", "admin"}
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Pool:       getPool(),
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key4",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user4@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key4", "secret"), M{
		"grant_type": PasswordGrant,
		"username":   "user4@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default admin", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
	})
}

func TestPasswordGrantInsufficientScope(t *testing.T) {
	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Pool:       getPool(),
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key5",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user5@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key5", "secret"), M{
		"grant_type": PasswordGrant,
		"username":   "user5@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// failing to get empty list of posts
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})
}

func TestCredentialsGrantAdditionalScope(t *testing.T) {
	policy.ClientCredentialsGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ClientCredentialsGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return []string{"default", "admin"}
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Pool:       getPool(),
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key6",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ClientCredentialsGrant},
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key6", "secret"), M{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default admin", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
	})
}

func TestCredentialsGrantInsufficientScope(t *testing.T) {
	policy.ClientCredentialsGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ClientCredentialsGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Pool:       getPool(),
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key7",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ClientCredentialsGrant},
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key7", "secret"), M{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// failing to get empty list of posts
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})
}

func TestImplicitGrantAdditionalScope(t *testing.T) {
	policy.ImplicitGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ImplicitGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return []string{"default", "admin"}
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Pool:       getPool(),
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key8",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ImplicitGrant},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user8@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/authorize", nil, M{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key8",
		"state":         "state1234",
		"scope":         "default",
		"username":      "user8@example.com",
		"password":      "secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		loc, err := url.Parse(r.Header().Get("Location"))
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
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
	})
}

func TestImplicitGrantInsufficientScope(t *testing.T) {
	policy.ImplicitGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ImplicitGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Pool:       getPool(),
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key9",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ImplicitGrant},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user9@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/authorize", nil, M{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key9",
		"state":         "state1234",
		"scope":         "default",
		"username":      "user9@example.com",
		"password":      "secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		loc, err := url.Parse(r.Header().Get("Location"))
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
	testRequest(server, "GET", "/posts", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.NotEmpty(t, r.Body.String())
	})
}

func TestGinAuthorizer(t *testing.T) {
	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(getPool(), policy, "auth")

	server, db := buildServer()
	server.GET("/foo", func(ctx echo.Context) error {
		return ctx.String(http.StatusOK, "OK")
	}, authenticator.EchoAuthorizer("default"))

	authenticator.Register(server)

	// create application
	saveModel(db, &Application{
		Name:       "Test Application",
		Key:        "key10",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(db, &User{
		Name:         "Test User",
		Email:        "user10@example.com",
		PasswordHash: hashPassword("secret"),
	})

	// missing auth
	testRequest(server, "GET", "/foo", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.Empty(t, r.Body.String())
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key10", "secret"), M{
		"grant_type": PasswordGrant,
		"username":   "user10@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		json, _ := gabs.ParseJSONBuffer(r.Body)
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
		assert.Equal(t, "default", json.Path("scope").Data().(string))
		assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

		token = json.Path("access_token").Data().(string)
	})

	// get empty list of posts
	testRequest(server, "GET", "/foo", M{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
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
