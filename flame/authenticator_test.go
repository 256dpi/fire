package flame

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/oauth2/v2/oauth2test"
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

		policy := DefaultPolicy("")
		policy.Grants = StaticGrants(true, true, true, true, true)

		policy.ClientFilter = func(Client, *http.Request) (bson.M, error) {
			return bson.M{"_id": bson.M{"$exists": true}}, nil
		}

		policy.ResourceOwnerFilter = func(Client, ResourceOwner, *http.Request) (bson.M, error) {
			return bson.M{"_id": bson.M{"$exists": true}}, nil
		}

		policy.GrantStrategy = func(_ Client, _ ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error) {
			if !allowedScope.Includes(scope) {
				return nil, ErrInvalidScope
			}

			if !scope.Includes(requiredScope) {
				return nil, ErrInvalidScope
			}

			return scope, nil
		}

		policy.ApproveStrategy = func(_ Client, _ ResourceOwner, _ GenericToken, scope oauth2.Scope) (oauth2.Scope, error) {
			if !allowedScope.Includes(scope) {
				return nil, ErrInvalidScope
			}

			if !scope.Includes(requiredScope) {
				return nil, ErrInvalidScope
			}

			return scope, nil
		}

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			t.Error(err)
		})

		redirectURIs := []string{"http://example.com/callback1", "http://example.com/callback2"}

		app1 := tester.Save(&Application{
			Name:         "Application 1",
			Key:          "app1",
			SecretHash:   mustHash(testPassword),
			RedirectURIs: redirectURIs,
		}).(*Application)

		app2 := tester.Save(&Application{
			Name:         "Application 2",
			Key:          "app2",
			RedirectURIs: redirectURIs,
		}).(*Application)

		user := tester.Save(&User{
			Name:         "User",
			Email:        "user@example.com",
			PasswordHash: mustHash(testPassword),
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

		validToken := tester.Save(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo"},
			Application: app1.ID(),
		}).(*Token)

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

		spec.InvalidToken = "foo"
		spec.UnknownToken = mustGenerateToken(policy, AccessToken, coal.New(), time.Now())
		spec.ValidToken = mustGenerateToken(policy, AccessToken, validToken.ID(), validToken.ExpiresAt)
		spec.ExpiredToken = mustGenerateToken(policy, AccessToken, expiredToken.ID(), expiredToken.ExpiresAt)
		spec.InsufficientToken = mustGenerateToken(policy, AccessToken, insufficientToken.ID(), insufficientToken.ExpiresAt)

		spec.PrimaryRedirectURI = redirectURIs[0]
		spec.SecondaryRedirectURI = redirectURIs[1]

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

		spec.UnknownRefreshToken = mustGenerateToken(policy, RefreshToken, coal.New(), time.Now())
		spec.ValidRefreshToken = mustGenerateToken(policy, RefreshToken, validRefreshToken.ID(), validRefreshToken.ExpiresAt)
		spec.ExpiredRefreshToken = mustGenerateToken(policy, RefreshToken, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		spec.InvalidAuthorizationCode = "foo"
		spec.UnknownAuthorizationCode = mustGenerateToken(policy, AuthorizationCode, coal.New(), time.Now())
		spec.ExpiredAuthorizationCode = mustGenerateToken(policy, AuthorizationCode, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		validToken = tester.Save(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
			User:        coal.P(user.ID()),
		}).(*Token)

		validBearerToken, _ := policy.GenerateJWT(validToken, app1, user)

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
		policy := DefaultPolicy("")

		authenticator := NewAuthenticator(tester.Store, policy, panicReporter)
		handler := newHandler(authenticator, false)

		application := tester.Save(&Application{
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
		policy := DefaultPolicy("")
		policy.Grants = StaticGrants(true, false, false, false, false)

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

		policy.ClientFilter = func(Client, *http.Request) (bson.M, error) {
			return nil, errors.New("foo")
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
		policy := DefaultPolicy("")
		policy.Grants = StaticGrants(true, false, false, false, false)

		var errs []string

		authenticator := NewAuthenticator(tester.Store, policy, func(err error) {
			errs = append(errs, err.Error())
		})
		handler := newHandler(authenticator, false)

		application := tester.Save(&Application{
			Key: "application",
		}).(*Application)

		policy.ResourceOwnerFilter = func(Client, ResourceOwner, *http.Request) (bson.M, error) {
			return nil, ErrInvalidFilter
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

		policy.ResourceOwnerFilter = func(Client, ResourceOwner, *http.Request) (bson.M, error) {
			return nil, errors.New("foo")
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
