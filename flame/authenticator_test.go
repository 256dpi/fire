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
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func panicReporter(err error) {
	panic(err)
}

func TestIntegration(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		var testPassword = "foo"
		var allowedScope = oauth2.ParseScope("foo bar")
		var requiredScope = oauth2.ParseScope("foo")

		p := DefaultPolicy("")

		p.PasswordGrant = true
		p.ClientCredentialsGrant = true
		p.ImplicitGrant = true
		p.AuthorizationCodeGrant = true

		p.ClientFilter = func(c Client, req *http.Request) (bson.M, error) {
			return bson.M{"_id": bson.M{"$exists": true}}, nil
		}

		p.ResourceOwnerFilter = func(ro ResourceOwner, req *http.Request) (bson.M, error) {
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

		p.ApproveStrategy = func(_ GenericToken, scope oauth2.Scope, _ Client, _ ResourceOwner) (oauth2.Scope, error) {
			if !allowedScope.Includes(scope) {
				return nil, ErrInvalidScope
			}

			if !scope.Includes(requiredScope) {
				return nil, ErrInvalidScope
			}

			return scope, nil
		}

		authenticator := NewAuthenticator(tester.Store, p, func(err error) {
			t.Error(err)
		})

		app1 := tester.Save(&Application{
			Name:         "Application 1",
			Key:          "app1",
			SecretHash:   mustHash(testPassword),
			RedirectURIs: []string{"http://example.com/callback1"},
		}).(*Application)

		app2 := tester.Save(&Application{
			Name:         "Application 2",
			Key:          "app2",
			RedirectURIs: []string{"http://example.com/callback2"},
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
		config.AuthorizationCodeGrantSupport = true
		config.RefreshTokenGrantSupport = true

		config.ConfidentialClientID = app1.Key
		config.ConfidentialClientSecret = testPassword
		config.PublicClientID = app2.Key

		config.ResourceOwnerUsername = user.Email
		config.ResourceOwnerPassword = testPassword

		config.InvalidScope = "baz"
		config.ValidScope = "foo bar"
		config.ExceedingScope = "foo bar baz"

		config.ExpectedExpiresIn = int(authenticator.policy.AccessTokenLifespan / time.Second)

		expiredToken := tester.Save(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(-authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo"},
			Application: app1.ID(),
		}).(*Token)

		insufficientToken := tester.Save(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{},
			Application: app1.ID(),
		}).(*Token)

		config.UnknownToken = mustGenerateToken(p, AccessToken, coal.New(), time.Now())
		config.ExpiredToken = mustGenerateToken(p, AccessToken, expiredToken.ID(), expiredToken.ExpiresAt)
		config.InsufficientToken = mustGenerateToken(p, AccessToken, insufficientToken.ID(), insufficientToken.ExpiresAt)

		config.PrimaryRedirectURI = "http://example.com/callback1"
		config.SecondaryRedirectURI = "http://example.com/callback2"

		validRefreshToken := tester.Save(&Token{
			Type:        RefreshToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.RefreshTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
		}).(*Token)

		expiredRefreshToken := tester.Save(&Token{
			Type:        RefreshToken,
			ExpiresAt:   time.Now().Add(-authenticator.policy.RefreshTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
		}).(*Token)

		config.UnknownRefreshToken = mustGenerateToken(p, RefreshToken, coal.New(), time.Now())
		config.ValidRefreshToken = mustGenerateToken(p, RefreshToken, validRefreshToken.ID(), validRefreshToken.ExpiresAt)
		config.ExpiredRefreshToken = mustGenerateToken(p, RefreshToken, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		config.InvalidAuthorizationCode = "foo"
		config.UnknownAuthorizationCode = mustGenerateToken(p, AuthorizationCode, coal.New(), time.Now())
		config.ExpiredAuthorizationCode = mustGenerateToken(p, AuthorizationCode, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		validToken := tester.Save(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.RefreshTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
			User:        coal.P(user.ID()),
		}).(*Token)

		validBearerToken, _ := p.GenerateJWT(validToken, app1, user)

		config.InvalidAuthorizationParams = map[string]string{
			"access_token": "foo",
		}

		config.ValidAuthorizationParams = map[string]string{
			"access_token": validBearerToken,
		}

		spec.Run(t, config)
	})
}

func TestPublicAccess(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(""), panicReporter)
		tester.Handler = newHandler(authenticator, false)

		tester.Request("GET", "api/protected", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, "OK", r.Body.String())
		})
	})
}

func TestContextKeys(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(""), panicReporter)
		tester.Handler = newHandler(authenticator, false)

		application := tester.Save(&Application{
			Key: "application",
		}).(*Application).ID()

		user := tester.Save(&User{
			Name:  "User",
			Email: "email@example.com",
		}).(*User).ID()

		accessToken := tester.Save(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Application: application,
			User:        &user,
		}).(*Token).ID()

		token := mustGenerateToken(authenticator.policy, AccessToken, accessToken, time.Now().Add(time.Hour))

		auth := authenticator.Authorizer("", true, true, true)

		tester.Handler.(*http.ServeMux).Handle("/api/info", auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, accessToken, r.Context().Value(AccessTokenContextKey).(*Token).ID())
			assert.Equal(t, application, r.Context().Value(ClientContextKey).(*Application).ID())
			assert.Equal(t, user, r.Context().Value(ResourceOwnerContextKey).(*User).ID())
		})))

		tester.Header["Authorization"] = "Bearer " + token
		tester.Request("GET", "api/info", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Code, tester.DebugRequest(rq, r))
		})
	})
}

func TestInvalidGrantType(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := DefaultPolicy("")

		authenticator := NewAuthenticator(tester.Store, policy, panicReporter)
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
	})
}

func TestInvalidResponseType(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := DefaultPolicy("")

		authenticator := NewAuthenticator(tester.Store, policy, panicReporter)
		handler := newHandler(authenticator, false)

		application := tester.Save(&Application{
			Key:          "application",
			RedirectURIs: []string{"https://example.com/"},
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
	})
}

func TestInvalidClientFilter(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := DefaultPolicy("")
		policy.PasswordGrant = true

		var errs []string

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			errs = append(errs, err.Error())
		})
		handler := newHandler(authenticator, false)

		application := tester.Save(&Application{
			Key: "application",
		}).(*Application)

		policy.ClientFilter = func(Client, *http.Request) (bson.M, error) {
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

		policy.ClientFilter = func(Client, *http.Request) (bson.M, error) {
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

		assert.Equal(t, []string{"foo"}, errs)
	})
}

func TestInvalidResourceOwnerFilter(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := DefaultPolicy("")
		policy.PasswordGrant = true

		var errs []string

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			errs = append(errs, err.Error())
		})
		handler := newHandler(authenticator, false)

		application := tester.Save(&Application{
			Key: "application",
		}).(*Application)

		policy.ResourceOwnerFilter = func(ResourceOwner, *http.Request) (bson.M, error) {
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

		policy.ResourceOwnerFilter = func(ResourceOwner, *http.Request) (bson.M, error) {
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

		assert.Equal(t, []string{"foo"}, errs)
	})
}

func mustGenerateToken(p *Policy, typ TokenType, id coal.ID, expiresAt time.Time) string {
	token := coal.Init(&Token{
		Base:      coal.Base{DocID: id},
		Type:      typ,
		ExpiresAt: expiresAt,
	}).(*Token)

	str, err := p.GenerateJWT(token, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}
