package flame

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/oauth2/v2/oauth2test"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

func panicReporter(err error) {
	panic(err)
}

func TestIntegration(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		var testPassword = "foo"
		var allowedScope = oauth2.ParseScope("foo bar")
		var requiredScope = oauth2.ParseScope("foo")

		policy := DefaultPolicy(testNotary)
		policy.Grants = StaticGrants(true, true, true, true, true)

		policy.ClientFilter = func(*Context, Client) (bson.M, error) {
			return bson.M{"_id": bson.M{"$exists": true}}, nil
		}

		policy.ResourceOwnerFilter = func(*Context, Client, ResourceOwner) (bson.M, error) {
			return bson.M{"_id": bson.M{"$exists": true}}, nil
		}

		policy.GrantStrategy = func(_ *Context, _ Client, _ ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error) {
			if !allowedScope.Includes(scope) {
				return nil, ErrInvalidScope.Wrap()
			}

			if !scope.Includes(requiredScope) {
				return nil, ErrInvalidScope.Wrap()
			}

			return scope, nil
		}

		policy.ApproveStrategy = func(_ *Context, _ Client, _ ResourceOwner, _ GenericToken, scope oauth2.Scope) (oauth2.Scope, error) {
			if !allowedScope.Includes(scope) {
				return nil, ErrInvalidScope.Wrap()
			}

			if !scope.Includes(requiredScope) {
				return nil, ErrInvalidScope.Wrap()
			}

			return scope, nil
		}

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			t.Error(err)
		})

		redirectURIs := []string{"http://example.com/callback1", "http://example.com/callback2"}

		app1 := tester.Insert(&Application{
			Name:         "Application 1",
			Key:          "app1",
			SecretHash:   heat.MustHash(testPassword),
			RedirectURIs: redirectURIs,
		}).(*Application)

		app2 := tester.Insert(&Application{
			Name:         "Application 2",
			Key:          "app2",
			RedirectURIs: redirectURIs,
		}).(*Application)

		user := tester.Insert(&User{
			Name:         "User",
			Email:        "user@example.com",
			PasswordHash: heat.MustHash(testPassword),
		}).(*User)

		spec := oauth2test.Default(newHandler(authenticator, true))

		spec.PasswordGrantSupport = true
		spec.ClientCredentialsGrantSupport = true
		spec.ImplicitGrantSupport = true
		spec.AuthorizationCodeGrantSupport = true
		spec.RefreshTokenGrantSupport = true

		spec.ConfidentialClientID = app1.Key
		spec.ConfidentialClientSecret = testPassword
		spec.PublicClientID = app2.Key

		spec.ResourceOwnerUsername = user.Email
		spec.ResourceOwnerPassword = testPassword

		spec.InvalidScope = "baz"
		spec.ValidScope = "foo bar"
		spec.ExceedingScope = "foo bar baz"

		spec.ExpectedExpiresIn = int(authenticator.policy.AccessTokenLifespan / time.Second)

		validToken := tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo"},
			Application: app1.ID(),
		}).(*Token)

		expiredToken := tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(-authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo"},
			Application: app1.ID(),
		}).(*Token)

		insufficientToken := tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{},
			Application: app1.ID(),
		}).(*Token)

		spec.InvalidToken = "foo"
		spec.UnknownToken = mustIssue(policy, AccessToken, coal.New(), time.Now())
		spec.ValidToken = mustIssue(policy, AccessToken, validToken.ID(), validToken.ExpiresAt)
		spec.ExpiredToken = mustIssue(policy, AccessToken, expiredToken.ID(), expiredToken.ExpiresAt)
		spec.InsufficientToken = mustIssue(policy, AccessToken, insufficientToken.ID(), insufficientToken.ExpiresAt)

		spec.PrimaryRedirectURI = redirectURIs[0]
		spec.SecondaryRedirectURI = redirectURIs[1]

		validRefreshToken := tester.Insert(&Token{
			Type:        RefreshToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.RefreshTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
		}).(*Token)

		expiredRefreshToken := tester.Insert(&Token{
			Type:        RefreshToken,
			ExpiresAt:   time.Now().Add(-authenticator.policy.RefreshTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
		}).(*Token)

		spec.UnknownRefreshToken = mustIssue(policy, RefreshToken, coal.New(), time.Now())
		spec.ValidRefreshToken = mustIssue(policy, RefreshToken, validRefreshToken.ID(), validRefreshToken.ExpiresAt)
		spec.ExpiredRefreshToken = mustIssue(policy, RefreshToken, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		spec.InvalidAuthorizationCode = "foo"
		spec.UnknownAuthorizationCode = mustIssue(policy, AuthorizationCode, coal.New(), time.Now())
		spec.ExpiredAuthorizationCode = mustIssue(policy, AuthorizationCode, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		validToken = tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
			User:        coal.P(user.ID()),
		}).(*Token)

		validBearerToken, _ := policy.Issue(validToken, app1, user)

		spec.InvalidAuthorizationParams = map[string]string{
			"access_token": "foo",
		}

		spec.ValidAuthorizationParams = map[string]string{
			"access_token": validBearerToken,
		}

		oauth2test.Run(t, spec)
	})
}

func TestPublicAccess(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(testNotary), panicReporter)
		tester.Handler = newHandler(authenticator, false)

		tester.Request("GET", "api/protected", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, "OK", r.Body.String())
		})
	})
}

func TestContextKeys(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(testNotary), panicReporter)
		tester.Handler = newHandler(authenticator, false)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application).ID()

		user := tester.Insert(&User{
			Name:     "User",
			Email:    "email@example.com",
			Password: "foo",
		}).(*User).ID()

		accessToken := tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Application: application,
			User:        &user,
		}).(*Token).ID()

		token := mustIssue(authenticator.policy, AccessToken, accessToken, time.Now().Add(time.Hour))

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
		policy := DefaultPolicy(testNotary)

		authenticator := NewAuthenticator(tester.Store, policy, panicReporter)
		handler := newHandler(authenticator, false)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		for _, gt := range []string{"password", "client_credentials", "authorization_code"} {
			oauth2test.Do(handler, &oauth2test.Request{
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
		policy := DefaultPolicy(testNotary)

		authenticator := NewAuthenticator(tester.Store, policy, panicReporter)
		handler := newHandler(authenticator, false)

		application := tester.Insert(&Application{
			Name:         "App",
			Key:          "application",
			RedirectURIs: []string{"https://example.com/"},
		}).(*Application)

		for _, rt := range []string{"token", "code"} {
			oauth2test.Do(handler, &oauth2test.Request{
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
		policy := DefaultPolicy(testNotary)
		policy.Grants = StaticGrants(true, false, false, false, false)

		var errs []string

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			errs = append(errs, err.Error())
		})
		handler := newHandler(authenticator, false)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		policy.ClientFilter = func(*Context, Client) (bson.M, error) {
			return nil, ErrInvalidFilter.Wrap()
		}

		oauth2test.Do(handler, &oauth2test.Request{
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

		policy.ClientFilter = func(*Context, Client) (bson.M, error) {
			return nil, xo.F("foo")
		}

		oauth2test.Do(handler, &oauth2test.Request{
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
		policy := DefaultPolicy(testNotary)
		policy.Grants = StaticGrants(true, false, false, false, false)

		var errs []string

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			errs = append(errs, err.Error())
		})
		handler := newHandler(authenticator, false)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		policy.ResourceOwnerFilter = func(*Context, Client, ResourceOwner) (bson.M, error) {
			return nil, ErrInvalidFilter.Wrap()
		}

		oauth2test.Do(handler, &oauth2test.Request{
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

		policy.ResourceOwnerFilter = func(*Context, Client, ResourceOwner) (bson.M, error) {
			return nil, xo.F("foo")
		}

		oauth2test.Do(handler, &oauth2test.Request{
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

func mustIssue(p *Policy, typ TokenType, id coal.ID, expiresAt time.Time) string {
	str, err := p.Issue(&Token{
		Base:      coal.B(id),
		Type:      typ,
		ExpiresAt: expiresAt,
	}, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}
