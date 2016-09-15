package oauth2

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/jsonapi"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"golang.org/x/crypto/bcrypt"
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
	store := getCleanStore()

	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model: &Post{},
		Store: store,
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
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key1",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(&User{
		Name:         "Test User",
		Email:        "user1@example.com",
		PasswordHash: hashPassword("secret"),
	})

	// TODO: Improve error message.

	// missing auth
	testRequest(server, "GET", "/posts", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "401",
				"detail": "An error occurred: The request could not be authorized"
			}]
		}`, r.Body.String())
	})

	// wrong secret
	testRequest(server, "POST", "/auth/token", basicAuth("key1", "secret"), map[string]string{
		"grant_type": PasswordGrant,
		"username":   "user1@example.com",
		"password":   "wrong-secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.JSONEq(t, `{
			"name": "invalid_request",
			"description": "The request is missing a required parameter, includes an invalid parameter value, includes a parameter more than once, or is otherwise malformed",
			"statusCode": 400
		}`, r.Body.String())
	})

	// wrong scope
	testRequest(server, "POST", "/auth/token", basicAuth("key1", "secret"), map[string]string{
		"grant_type": PasswordGrant,
		"username":   "user1@example.com",
		"password":   "secret",
		"scope":      "wrong",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.JSONEq(t, `{
			"name":"invalid_scope",
			"description":"The requested scope is invalid, unknown, or malformed",
			"statusCode":400
		}`, r.Body.String())
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key1", "secret"), map[string]string{
		"grant_type": PasswordGrant,
		"username":   "user1@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.True(t, accessToken.OwnerID.Valid())

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
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

func TestClientCredentialsGrant(t *testing.T) {
	store := getCleanStore()

	policy.ClientCredentialsGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ClientCredentialsGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model: &Post{},
		Store: store,
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
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key2",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{ClientCredentialsGrant},
	})

	// missing auth
	testRequest(server, "GET", "/posts", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "401",
				"detail": "An error occurred: The request could not be authorized"
			}]
		}`, r.Body.String())
	})

	// wrong secret
	testRequest(server, "POST", "/auth/token", basicAuth("key2", "wrong-secret"), map[string]string{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.JSONEq(t, `{
			"name": "invalid_client",
			"description": "Client authentication failed (e.g., unknown client, no client authentication included, or unsupported authentication method)",
			"statusCode": 400
		}`, r.Body.String())
	})

	// wrong scope
	testRequest(server, "POST", "/auth/token", basicAuth("key2", "secret"), map[string]string{
		"grant_type": ClientCredentialsGrant,
		"scope":      "wrong",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Code)
		assert.JSONEq(t, `{
			"name": "invalid_scope",
			"description": "The requested scope is invalid, unknown, or malformed",
			"statusCode": 400
		}`, r.Body.String())
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key2", "secret"), map[string]string{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.Nil(t, accessToken.OwnerID)

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
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

func TestImplicitGrant(t *testing.T) {
	store := getCleanStore()

	policy.ImplicitGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ImplicitGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model: &Post{},
		Store: store,
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
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key3",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{ImplicitGrant},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(&User{
		Name:         "Test User",
		Email:        "user3@example.com",
		PasswordHash: hashPassword("secret"),
	})

	// missing auth
	testRequest(server, "GET", "/posts", nil, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "401",
				"detail": "An error occurred: The request could not be authorized"
			}]
		}`, r.Body.String())
	})

	// wrong secret
	testRequest(server, "POST", "/auth/authorize", nil, map[string]string{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key3",
		"state":         "state1234",
		"scope":         "default",
		"username":      "user3@example.com",
		"password":      "wrong-secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusFound, r.Code)
		assert.Empty(t, r.Body.String())
		assert.Equal(t, "https://0.0.0.0:8080/auth/callback?error=acccess_denied&error_description=The+resource+owner+or+authorization+server+denied+the+request&state=state1234", r.Header().Get("Location"))
	})

	// wrong scope
	testRequest(server, "POST", "/auth/authorize", nil, map[string]string{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key3",
		"state":         "state1234",
		"scope":         "wrong",
		"username":      "user3@example.com",
		"password":      "secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusFound, r.Code)
		assert.Empty(t, r.Body.String())
		assert.Equal(t, "https://0.0.0.0:8080/auth/callback?error=invalid_scope&error_description=The+requested+scope+is+invalid%2C+unknown%2C+or+malformed&state=state1234", r.Header().Get("Location"))
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/authorize", nil, map[string]string{
		"response_type": "token",
		"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
		"client_id":     "key3",
		"state":         "state1234",
		"scope":         "default",
		"username":      "user3@example.com",
		"password":      "secret",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.True(t, accessToken.OwnerID.Valid())

		loc, err := url.Parse(r.Header().Get("Location"))
		assert.NoError(t, err)
		query, err := url.ParseQuery(loc.Fragment)
		assert.NoError(t, err)

		token = query.Get("access_token")

		assert.Equal(t, http.StatusFound, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", query.Get("expires_in"))
		assert.Equal(t, "default", query.Get("scope"))
		assert.Equal(t, "bearer", query.Get("token_type"))
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
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

func TestPasswordGrantAdditionalScope(t *testing.T) {
	store := getCleanStore()

	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return []string{"default", "admin"}
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key4",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(&User{
		Name:         "Test User",
		Email:        "user4@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key4", "secret"), map[string]string{
		"grant_type": PasswordGrant,
		"username":   "user4@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default", "admin"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.True(t, accessToken.OwnerID.Valid())

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default admin", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
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
	store := getCleanStore()

	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Store:      store,
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key5",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(&User{
		Name:         "Test User",
		Email:        "user5@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key5", "secret"), map[string]string{
		"grant_type": PasswordGrant,
		"username":   "user5@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.True(t, accessToken.OwnerID.Valid())

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// failing to get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "401",
				"detail": "An error occurred: The requested scope is invalid, unknown, or malformed"
			}]
		}`, r.Body.String())
	})
}

func TestCredentialsGrantAdditionalScope(t *testing.T) {
	store := getCleanStore()

	policy.ClientCredentialsGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ClientCredentialsGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return []string{"default", "admin"}
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key6",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ClientCredentialsGrant},
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key6", "secret"), map[string]string{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default", "admin"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.Nil(t, accessToken.OwnerID)

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default admin", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
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
	store := getCleanStore()

	policy.ClientCredentialsGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ClientCredentialsGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.Nil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Store:      store,
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key7",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ClientCredentialsGrant},
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/token", basicAuth("key7", "secret"), map[string]string{
		"grant_type": ClientCredentialsGrant,
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.Nil(t, accessToken.OwnerID)

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// failing to get empty list of posts
	testRequest(server, "GET", "/posts", map[string]string{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "401",
				"detail": "An error occurred: The requested scope is invalid, unknown, or malformed"
			}]
		}`, r.Body.String())
	})
}

func TestImplicitGrantAdditionalScope(t *testing.T) {
	store := getCleanStore()

	policy.ImplicitGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ImplicitGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return []string{"default", "admin"}
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default", "admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key8",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ImplicitGrant},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(&User{
		Name:         "Test User",
		Email:        "user8@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/authorize", nil, map[string]string{
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
	testRequest(server, "GET", "/posts", map[string]string{
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
	store := getCleanStore()

	policy.ImplicitGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, ImplicitGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer(&jsonapi.Controller{
		Model:      &Post{},
		Store:      store,
		Authorizer: authenticator.Authorizer("admin"),
	})

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key9",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default", "admin"},
		GrantTypes: []string{ImplicitGrant},
		Callbacks:  []string{"https://0.0.0.0:8080/auth/callback"},
	})

	// create user
	saveModel(&User{
		Name:         "Test User",
		Email:        "user9@example.com",
		PasswordHash: hashPassword("secret"),
	})

	var token string

	// get access token
	testRequest(server, "POST", "/auth/authorize", nil, map[string]string{
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
	testRequest(server, "GET", "/posts", map[string]string{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusUnauthorized, r.Code)
		assert.JSONEq(t, `{
			"errors": [{
				"status": "401",
				"detail": "An error occurred: The requested scope is invalid, unknown, or malformed"
			}]
		}`, r.Body.String())
	})
}

func TestEchoAuthorizer(t *testing.T) {
	store := getCleanStore()

	policy.PasswordGrant = true

	policy.GrantStrategy = func(req *GrantRequest) []string {
		assert.Equal(t, PasswordGrant, req.GrantType)
		assert.Equal(t, []string{"default"}, req.RequestedScopes)
		assert.NotNil(t, req.Client)
		assert.NotNil(t, req.Owner)

		return req.RequestedScopes
	}

	authenticator := New(store, policy, "auth")

	server := buildServer()
	server.GET("/foo", func(ctx echo.Context) error {
		return ctx.String(http.StatusOK, "OK")
	}, authenticator.EchoAuthorizer("default"))

	authenticator.Register(server)

	// create application
	saveModel(&Application{
		Name:       "Test Application",
		Key:        "key10",
		SecretHash: hashPassword("secret"),
		Scopes:     []string{"default"},
		GrantTypes: []string{PasswordGrant},
	})

	// create user
	saveModel(&User{
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
	testRequest(server, "POST", "/auth/token", basicAuth("key10", "secret"), map[string]string{
		"grant_type": PasswordGrant,
		"username":   "user10@example.com",
		"password":   "secret",
		"scope":      "default",
	}, func(r *httptest.ResponseRecorder, rq engine.Request) {
		accessToken := findLastModel(&AccessToken{}).(*AccessToken)
		assert.Equal(t, []string{"default"}, accessToken.GrantedScopes)
		assert.True(t, accessToken.ClientID.Valid())
		assert.True(t, accessToken.OwnerID.Valid())

		token = gjson.Get(r.Body.String(), "access_token").String()

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, string(accessToken.Signature), strings.Split(token, ".")[1])
		assert.Equal(t, "3600", gjson.Get(r.Body.String(), "expires_in").String())
		assert.Equal(t, "default", gjson.Get(r.Body.String(), "scope").String())
		assert.Equal(t, "bearer", gjson.Get(r.Body.String(), "token_type").String())
	})

	// get empty list of posts
	testRequest(server, "GET", "/foo", map[string]string{
		"Authorization": "Bearer " + token,
	}, nil, func(r *httptest.ResponseRecorder, rq engine.Request) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, "OK", r.Body.String())
	})
}

func TestAuthenticatorInspect(t *testing.T) {
	p := DefaultPolicy()
	p.Secret = []byte("abcd1234abcd1234")
	p.PasswordGrant = true

	a := New(getCleanStore(), p, "auth")

	assert.Equal(t, fire.ComponentInfo{
		Name: "OAuth2 Authenticator",
		Settings: fire.Map{
			"Prefix":                         "auth",
			"Allow Password Grant":           "true",
			"Allow Client Credentials Grant": "false",
			"Allow Implicit Grant":           "false",
			"Token Lifespan":                 "1h0m0s",
			"Access Token Model":             "oauth2.AccessToken",
			"Client Model":                   "oauth2.Application",
			"Owner Model":                    "oauth2.User",
		},
	}, a.Inspect())
}
