package flame

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/spec"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestIntegration(t *testing.T) {
	tester.Clean()

	var testPassword = "foo"
	var allowedScope = oauth2.ParseScope("foo bar")
	var requiredScope = oauth2.ParseScope("foo")

	p := DefaultPolicy("")

	p.PasswordGrant = true
	p.ClientCredentialsGrant = true
	p.ImplicitGrant = true

	p.Filter = func(ro ResourceOwner, req *http.Request) (bson.M, error) {
		return bson.M{"_id": bson.M{"$exists": true}}, nil
	}

	p.GrantStrategy = func(scope oauth2.Scope, _ Client, _ ResourceOwner) (oauth2.Scope, error) {
		if !allowedScope.Includes(scope) {
			return nil, ErrInvalidScope
		}

		if !scope.Includes(requiredScope) {
			return nil, ErrInvalidScope
		}

		return scope, nil
	}

	authenticator := NewAuthenticator(tester.Store, p)
	authenticator.Reporter = func(err error) {
		t.Error(err)
	}

	app1 := tester.Save(&Application{
		Name:        "Application 1",
		Key:         "app1",
		SecretHash:  mustHash(testPassword),
		RedirectURL: "http://example.com/callback1",
	}).(*Application)

	app2 := tester.Save(&Application{
		Name:        "Application 2",
		Key:         "app2",
		SecretHash:  mustHash(testPassword),
		RedirectURL: "http://example.com/callback2",
	}).(*Application)

	user := tester.Save(&User{
		Name:         "User",
		Email:        "user@example.com",
		PasswordHash: mustHash(testPassword),
	}).(*User)

	config := spec.Default(newHandler(authenticator, true))

	config.PasswordGrantSupport = true
	config.ClientCredentialsGrantSupport = true
	config.ImplicitGrantSupport = true
	config.RefreshTokenGrantSupport = true

	config.PrimaryClientID = app1.Key
	config.PrimaryClientSecret = testPassword
	config.SecondaryClientID = app2.Key
	config.SecondaryClientSecret = testPassword

	config.ResourceOwnerUsername = user.Email
	config.ResourceOwnerPassword = testPassword

	config.InvalidScope = "baz"
	config.ValidScope = "foo bar"
	config.ExceedingScope = "foo bar baz"

	config.ExpectedExpiresIn = int(authenticator.policy.AccessTokenLifespan / time.Second)

	expiredToken := tester.Save(&AccessToken{
		ExpiresAt: time.Now().Add(-authenticator.policy.AccessTokenLifespan),
		Scope:     []string{"foo"},
		Client:    app1.ID(),
	}).(*AccessToken)

	insufficientToken := tester.Save(&AccessToken{
		ExpiresAt: time.Now().Add(authenticator.policy.AccessTokenLifespan),
		Scope:     []string{},
		Client:    app1.ID(),
	}).(*AccessToken)

	config.UnknownToken = mustGenerateAccessToken(p, bson.NewObjectId(), time.Now())
	config.ExpiredToken = mustGenerateAccessToken(p, expiredToken.ID(), expiredToken.ExpiresAt)
	config.InsufficientToken = mustGenerateAccessToken(p, insufficientToken.ID(), insufficientToken.ExpiresAt)

	config.PrimaryRedirectURI = "http://example.com/callback1"
	config.SecondaryRedirectURI = "http://example.com/callback2"

	validRefreshToken := tester.Save(&RefreshToken{
		ExpiresAt: time.Now().Add(authenticator.policy.RefreshTokenLifespan),
		Scope:     []string{"foo", "bar"},
		Client:    app1.ID(),
	}).(*RefreshToken)

	expiredRefreshToken := tester.Save(&RefreshToken{
		ExpiresAt: time.Now().Add(-authenticator.policy.RefreshTokenLifespan),
		Scope:     []string{"foo", "bar"},
		Client:    app1.ID(),
	}).(*RefreshToken)

	config.UnknownRefreshToken = mustGenerateRefreshToken(p, bson.NewObjectId(), time.Now())
	config.ValidRefreshToken = mustGenerateRefreshToken(p, validRefreshToken.ID(), validRefreshToken.ExpiresAt)
	config.ExpiredRefreshToken = mustGenerateRefreshToken(p, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

	config.InvalidAuthorizationParams = map[string]string{
		"username": user.Email,
		"password": "",
	}

	config.ValidAuthorizationParams = map[string]string{
		"username": user.Email,
		"password": testPassword,
	}

	spec.Run(t, config)
}

func TestPublicAccess(t *testing.T) {
	tester.Clean()

	authenticator := NewAuthenticator(tester.Store, DefaultPolicy(""))
	tester.Handler = newHandler(authenticator, false)

	tester.Request("GET", "api/protected", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, "OK", r.Body.String())
	})
}

func TestContextKeys(t *testing.T) {
	tester.Clean()

	authenticator := NewAuthenticator(tester.Store, DefaultPolicy(""))
	tester.Handler = newHandler(authenticator, false)

	application := tester.Save(&Application{
		Key: "application",
	}).(*Application).ID()

	user := tester.Save(&User{
		Name:  "User",
		Email: "email@example.com",
	}).(*User).ID()

	accessToken := tester.Save(&AccessToken{
		ExpiresAt:     time.Now().Add(authenticator.policy.AccessTokenLifespan),
		Client:        application,
		ResourceOwner: &user,
	}).(*AccessToken).ID()

	token := mustGenerateAccessToken(authenticator.policy, accessToken, time.Now().Add(time.Hour))

	auth := authenticator.Authorizer("", true, true, true)

	tester.Handler.(*http.ServeMux).Handle("/api/info", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, accessToken, r.Context().Value(AccessTokenContextKey).(*AccessToken).ID())
		assert.Equal(t, application, r.Context().Value(ClientContextKey).(*Application).ID())
		assert.Equal(t, user, r.Context().Value(ResourceOwnerContextKey).(*User).ID())
	})))

	tester.Header["Authorization"] = "Bearer " + token
	tester.Request("GET", "api/info", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Code, tester.DebugRequest(rq, r))
	})
}

func TestInvalidGrantType(t *testing.T) {
	tester.Clean()

	policy := DefaultPolicy("")

	authenticator := NewAuthenticator(tester.Store, policy)
	handler := newHandler(authenticator, false)

	application := tester.Save(&Application{
		Key: "application",
	}).(*Application)

	for _, gt := range []string{"password", "client_credentials", "authorization_code"} {
		spec.Do(handler, &spec.Request{
			Method:   "POST",
			Path:     "/oauth2/token",
			Username: application.Key,
			Password: application.Secret,
			Form: map[string]string{
				"grant_type": gt,
				"username":   "foo",
				"password":   "bar",
				"scope":      "",
			},
			Callback: func(r *httptest.ResponseRecorder, rq *http.Request) {
				assert.Equal(t, http.StatusBadRequest, r.Code)
				assert.JSONEq(t, r.Body.String(), `{
				"error": "unsupported_grant_type"
			}`)
			},
		})
	}
}

func TestInvalidResponseType(t *testing.T) {
	tester.Clean()

	policy := DefaultPolicy("")

	authenticator := NewAuthenticator(tester.Store, policy)
	handler := newHandler(authenticator, false)

	application := tester.Save(&Application{
		Key:         "application",
		RedirectURL: "https://example.com/",
	}).(*Application)

	for _, rt := range []string{"token", "code"} {
		spec.Do(handler, &spec.Request{
			Method:   "POST",
			Path:     "/oauth2/authorize",
			Username: application.Key,
			Password: application.Secret,
			Form: map[string]string{
				"response_type": rt,
				"client_id":     application.Key,
				"redirect_uri":  "https://example.com/",
				"scope":         "",
				"state":         "xyz",
			},
			Callback: func(r *httptest.ResponseRecorder, rq *http.Request) {
				assert.Equal(t, http.StatusBadRequest, r.Code)
				assert.JSONEq(t, r.Body.String(), `{
				"error": "unsupported_response_type"
			}`)
			},
		})
	}
}

func TestInvalidFilter(t *testing.T) {
	tester.Clean()

	policy := DefaultPolicy("")
	policy.PasswordGrant = true

	authenticator := NewAuthenticator(tester.Store, policy)
	handler := newHandler(authenticator, false)

	application := tester.Save(&Application{
		Key: "application",
	}).(*Application)

	policy.Filter = func(ResourceOwner, *http.Request) (bson.M, error) {
		return nil, ErrInvalidFilter
	}

	spec.Do(handler, &spec.Request{
		Method:   "POST",
		Path:     "/oauth2/token",
		Username: application.Key,
		Password: application.Secret,
		Form: map[string]string{
			"grant_type": "password",
			"username":   "foo",
			"password":   "bar",
			"scope":      "",
		},
		Callback: func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Code)
			assert.JSONEq(t, r.Body.String(), `{
				"error": "invalid_request",
				"error_description": "invalid filter"
			}`)
		},
	})

	policy.Filter = func(ResourceOwner, *http.Request) (bson.M, error) {
		return nil, errors.New("foo")
	}

	spec.Do(handler, &spec.Request{
		Method:   "POST",
		Path:     "/oauth2/token",
		Username: application.Key,
		Password: application.Secret,
		Form: map[string]string{
			"grant_type": "password",
			"username":   "foo",
			"password":   "bar",
			"scope":      "",
		},
		Callback: func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusInternalServerError, r.Code)
			assert.JSONEq(t, r.Body.String(), `{
				"error": "server_error"
			}`)
		},
	})
}

func mustGenerateAccessToken(p *Policy, id bson.ObjectId, expiresAt time.Time) string {
	str, err := p.GenerateToken(id, time.Now(), expiresAt, nil, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}

func mustGenerateRefreshToken(p *Policy, id bson.ObjectId, expiresAt time.Time) string {
	str, err := p.GenerateToken(id, time.Now(), expiresAt, nil, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}
