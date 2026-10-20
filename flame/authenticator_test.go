package flame

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/256dpi/fire/stick"
)

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

		policy.backTrackIssuedFromExpiry = true

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
		spec.UnknownToken = mustIssue(policy, AccessToken, coal.New(), time.Now().Add(authenticator.policy.AccessTokenLifespan))
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

		spec.UnknownRefreshToken = mustIssue(policy, RefreshToken, coal.New(), time.Now().Add(authenticator.policy.RefreshTokenLifespan))
		spec.ValidRefreshToken = mustIssue(policy, RefreshToken, validRefreshToken.ID(), validRefreshToken.ExpiresAt)
		spec.ExpiredRefreshToken = mustIssue(policy, RefreshToken, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		spec.InvalidAuthorizationCode = "foo"
		spec.UnknownAuthorizationCode = mustIssue(policy, AuthorizationCode, coal.New(), time.Now().Add(authenticator.policy.AuthorizationCodeLifespan))
		spec.ExpiredAuthorizationCode = mustIssue(policy, AuthorizationCode, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

		validToken = tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(authenticator.policy.AccessTokenLifespan),
			Scope:       []string{"foo", "bar"},
			Application: app1.ID(),
			User:        stick.P(user.ID()),
		}).(*Token)

		validBearerToken, _ := policy.Issue(nil, validToken, app1, user)

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
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(testNotary), xo.Crash)
		tester.Handler = newHandler(authenticator, false)

		tester.Request("GET", "api/protected", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, "OK", r.Body.String())
		})
	})
}

func TestContextKeys(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(testNotary), xo.Crash)
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

		auth := authenticator.Authorizer(nil, true, true, true)

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

		authenticator := NewAuthenticator(tester.Store, policy, xo.Crash)
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

		authenticator := NewAuthenticator(tester.Store, policy, xo.Crash)
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
	str, err := p.Issue(nil, &Token{
		Base:      coal.B(id),
		Type:      typ,
		ExpiresAt: expiresAt,
	}, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}

type keyCapture struct {
	key           GenericKey
	client        Client
	resourceOwner ResourceOwner
	accessToken   GenericToken
}

func newKeyHandler(auth *Authenticator, loadClient, loadResourceOwner bool, capture *keyCapture) http.Handler {
	authorizer := auth.Authorizer([]string{"foo"}, true, loadClient, loadResourceOwner)
	return authorizer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*capture = keyCapture{}
		capture.key, _ = r.Context().Value(KeyContextKey).(GenericKey)
		capture.client, _ = r.Context().Value(ClientContextKey).(Client)
		capture.resourceOwner, _ = r.Context().Value(ResourceOwnerContextKey).(ResourceOwner)
		capture.accessToken, _ = r.Context().Value(AccessTokenContextKey).(GenericToken)
		_, _ = w.Write([]byte("OK"))
	}))
}

func keyRequest(handler http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func keyPolicy() *Policy {
	policy := DefaultPolicy(testNotary)
	policy.Key = &testKey{}
	policy.KeyPrefix = "tk_"
	return policy
}

func insertKey(t *testing.T, tester *fire.Tester, key *testKey) (*testKey, string) {
	key.Base = coal.B()
	secret, err := keyPolicy().GenerateKey(key)
	assert.NoError(t, err)
	tester.Insert(key)
	return key, secret
}

func TestKeyAuthorizer(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		var errs []string
		authenticator := NewAuthenticator(tester.Store, keyPolicy(), func(err error) {
			errs = append(errs, err.Error())
		})

		var capture keyCapture
		handler := newKeyHandler(authenticator, true, true, &capture)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		user := tester.Insert(&User{
			Name:     "User",
			Email:    "user@example.com",
			Password: "foo",
		}).(*User)

		/* keys referencing a client and resource owner */

		fullKey, fullSecret := insertKey(t, tester, &testKey{
			Scope:       []string{"foo", "bar"},
			Application: stick.P(application.ID()),
			User:        stick.P(user.ID()),
		})

		res := keyRequest(handler, fullSecret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, fullKey.ID(), capture.key.ID())
		assert.Equal(t, application.ID(), capture.client.ID())
		assert.Equal(t, user.ID(), capture.resourceOwner.ID())
		assert.Nil(t, capture.accessToken)

		/* keys referencing nothing */

		plainKey, plainSecret := insertKey(t, tester, &testKey{
			Scope: []string{"foo"},
		})

		res = keyRequest(handler, plainSecret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, plainKey.ID(), capture.key.ID())
		assert.Nil(t, capture.client)
		assert.Nil(t, capture.resourceOwner)
		assert.Nil(t, capture.accessToken)

		/* keys referencing only a resource owner */

		userKey, userSecret := insertKey(t, tester, &testKey{
			Scope: []string{"foo"},
			User:  stick.P(user.ID()),
		})

		res = keyRequest(handler, userSecret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, userKey.ID(), capture.key.ID())
		assert.Nil(t, capture.client)
		assert.Equal(t, user.ID(), capture.resourceOwner.ID())

		/* keys that have not yet expired */

		_, futureSecret := insertKey(t, tester, &testKey{
			Scope:     []string{"foo"},
			ExpiresAt: stick.P(time.Now().Add(time.Hour)),
		})

		res = keyRequest(handler, futureSecret)
		assert.Equal(t, http.StatusOK, res.Code)

		/* insufficient scope */

		_, barSecret := insertKey(t, tester, &testKey{
			Scope: []string{"bar"},
		})

		res = keyRequest(handler, barSecret)
		assert.Equal(t, http.StatusForbidden, res.Code)
		assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error="insufficient_scope"`)

		/* unknown and tampered keys */

		for _, secret := range []string{"tk_unknown", fullSecret + "x", fullSecret[:len(fullSecret)-1]} {
			res = keyRequest(handler, secret)
			assert.Equal(t, http.StatusUnauthorized, res.Code)
			assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error="invalid_token"`)
			assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="unknown key"`)
		}

		/* keys without the prefix are verified as access tokens */

		for _, secret := range []string{fullSecret[len("tk_"):], "ok_" + fullSecret[len("tk_"):]} {
			res = keyRequest(handler, secret)
			assert.Equal(t, http.StatusUnauthorized, res.Code)
			assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="malformed bearer token"`)
		}

		/* expired key */

		expiredKey, expiredSecret := insertKey(t, tester, &testKey{
			Scope:     []string{"foo"},
			ExpiresAt: stick.P(time.Now().Add(-time.Hour)),
		})

		res = keyRequest(handler, expiredSecret)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="expired key"`)
		assert.Nil(t, tester.Fetch(&testKey{}, expiredKey.ID()).(*testKey).Used)

		/* missing resource owner */

		_, missingUserSecret := insertKey(t, tester, &testKey{
			Scope: []string{"foo"},
			User:  stick.P(coal.New()),
		})

		res = keyRequest(handler, missingUserSecret)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="missing resource owner"`)

		/* missing client */

		_, missingClientSecret := insertKey(t, tester, &testKey{
			Scope:       []string{"foo"},
			Application: stick.P(coal.New()),
		})

		assert.Empty(t, errs)
		res = keyRequest(handler, missingClientSecret)
		assert.Equal(t, http.StatusInternalServerError, res.Code)
		assert.Equal(t, []string{"missing client"}, errs)

		/* revoked key */

		tester.Delete(plainKey)

		res = keyRequest(handler, plainSecret)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="unknown key"`)
	})
}

func TestKeyAuthorizerUse(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, keyPolicy(), xo.Crash)

		var capture keyCapture
		handler := newKeyHandler(authenticator, true, true, &capture)

		key, secret := insertKey(t, tester, &testKey{
			Scope: []string{"foo"},
		})
		assert.Nil(t, key.Used)

		// first use is recorded
		res := keyRequest(handler, secret)
		assert.Equal(t, http.StatusOK, res.Code)
		used := capture.key.(*testKey).Used
		assert.NotNil(t, used)
		assert.WithinDuration(t, time.Now(), *used, time.Second)
		stored := tester.Fetch(&testKey{}, key.ID()).(*testKey).Used
		assert.NotNil(t, stored)
		assert.WithinDuration(t, *used, *stored, time.Millisecond)

		// recent use is not recorded again
		recent := time.Now().Add(-KeyUseInterval / 2).Round(time.Millisecond)
		tester.Update(key, bson.M{"$set": bson.M{"Used": recent}})
		res = keyRequest(handler, secret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, recent, capture.key.(*testKey).Used.Local())
		assert.Equal(t, recent, tester.Fetch(&testKey{}, key.ID()).(*testKey).Used.Local())

		// older use is recorded again
		tester.Update(key, bson.M{"$set": bson.M{"Used": time.Now().Add(-KeyUseInterval * 2)}})
		res = keyRequest(handler, secret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.WithinDuration(t, time.Now(), *capture.key.(*testKey).Used, time.Second)
		assert.WithinDuration(t, time.Now(), *tester.Fetch(&testKey{}, key.ID()).(*testKey).Used, time.Second)
	})
}

func TestKeyAuthorizerWithoutUse(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := DefaultPolicy(testNotary)
		policy.Key = &testMinimalKey{}
		policy.KeyPrefix = "tk_"

		authenticator := NewAuthenticator(tester.Store, policy, xo.Crash)

		var capture keyCapture
		handler := newKeyHandler(authenticator, true, true, &capture)

		key := &testMinimalKey{Base: coal.B(), Scope: []string{"foo"}}
		secret, err := policy.GenerateKey(key)
		assert.NoError(t, err)
		tester.Insert(key)

		res := keyRequest(handler, secret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, key.ID(), capture.key.ID())
	})
}

func TestKeyAuthorizerLoading(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, keyPolicy(), xo.Crash)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		user := tester.Insert(&User{
			Name:     "User",
			Email:    "user@example.com",
			Password: "foo",
		}).(*User)

		key, secret := insertKey(t, tester, &testKey{
			Scope:       []string{"foo"},
			Application: stick.P(application.ID()),
			User:        stick.P(user.ID()),
		})

		var capture keyCapture

		res := keyRequest(newKeyHandler(authenticator, false, false, &capture), secret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, key.ID(), capture.key.ID())
		assert.Nil(t, capture.client)
		assert.Nil(t, capture.resourceOwner)

		res = keyRequest(newKeyHandler(authenticator, true, false, &capture), secret)
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, key.ID(), capture.key.ID())
		assert.Equal(t, application.ID(), capture.client.ID())
		assert.Nil(t, capture.resourceOwner)
	})
}

func TestKeyAuthorizerAccessTokens(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := keyPolicy()
		authenticator := NewAuthenticator(tester.Store, policy, xo.Crash)

		var capture keyCapture
		handler := newKeyHandler(authenticator, true, true, &capture)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		accessToken := tester.Insert(&Token{
			Type:        AccessToken,
			ExpiresAt:   time.Now().Add(time.Hour),
			Scope:       []string{"foo"},
			Application: application.ID(),
		}).(*Token)

		// access tokens are still verified as such
		res := keyRequest(handler, mustIssue(policy, AccessToken, accessToken.ID(), accessToken.ExpiresAt))
		assert.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, accessToken.ID(), capture.accessToken.ID())
		assert.Equal(t, application.ID(), capture.client.ID())
		assert.Nil(t, capture.key)

		// tokens without the prefix are not verified as keys
		res = keyRequest(handler, "foo")
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="malformed bearer token"`)
	})
}

func TestKeyAuthorizerWithoutKeyModel(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		authenticator := NewAuthenticator(tester.Store, DefaultPolicy(testNotary), xo.Crash)

		var capture keyCapture
		handler := newKeyHandler(authenticator, true, true, &capture)

		_, secret := insertKey(t, tester, &testKey{
			Scope: []string{"foo"},
		})

		res := keyRequest(handler, secret)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
		assert.Contains(t, res.Header().Get("WWW-Authenticate"), `error_description="malformed bearer token"`)
		assert.Nil(t, capture.key)
	})
}

func TestNewAuthenticatorKeyModel(t *testing.T) {
	assert.NotPanics(t, func() {
		NewAuthenticator(lungoStore, keyPolicy(), xo.Crash)
	})

	assert.PanicsWithValue(t, "flame: missing key prefix", func() {
		policy := keyPolicy()
		policy.KeyPrefix = ""
		NewAuthenticator(lungoStore, policy, xo.Crash)
	})

	assert.PanicsWithValue(t, `flame: expected "flame-key-used" field to be a time pointer`, func() {
		policy := keyPolicy()
		policy.Key = &testInvalidUsedKey{}
		NewAuthenticator(lungoStore, policy, xo.Crash)
	})
}

func TestDefaultKeyAuthorizer(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		policy := DefaultPolicy(testNotary)
		policy.Key = &Key{}
		policy.KeyPrefix = "fk_"

		authenticator := NewAuthenticator(tester.Store, policy, xo.Crash)

		var capture keyCapture
		handler := newKeyHandler(authenticator, true, true, &capture)

		application := tester.Insert(&Application{
			Name: "App",
			Key:  "application",
		}).(*Application)

		user := tester.Insert(&User{
			Name:     "User",
			Email:    "user@example.com",
			Password: "foo",
		}).(*User)

		for _, item := range []struct {
			name        string
			application *coal.ID
			user        *coal.ID
		}{
			{name: "application", application: stick.P(application.ID())},
			{name: "user", user: stick.P(user.ID())},
			{name: "both", application: stick.P(application.ID()), user: stick.P(user.ID())},
		} {
			t.Run(item.name, func(t *testing.T) {
				key := &Key{
					Base:        coal.B(),
					Name:        item.name,
					Scope:       []string{"foo"},
					Application: item.application,
					User:        item.user,
				}

				// generate secret
				secret, err := policy.GenerateKey(key)
				assert.NoError(t, err)
				assert.Equal(t, secret, key.Secret)
				assert.True(t, strings.HasPrefix(key.Fingerprint, "fk_"))
				tester.Insert(key)

				// secret is not stored
				stored := tester.Fetch(&Key{}, key.ID()).(*Key)
				assert.Empty(t, stored.Secret)
				assert.Nil(t, stored.Used)

				// authorize
				res := keyRequest(handler, secret)
				assert.Equal(t, http.StatusOK, res.Code)
				assert.Equal(t, key.ID(), capture.key.ID())
				assert.Nil(t, capture.accessToken)
				if item.application != nil {
					assert.Equal(t, application.ID(), capture.client.ID())
				} else {
					assert.Nil(t, capture.client)
				}
				if item.user != nil {
					assert.Equal(t, user.ID(), capture.resourceOwner.ID())
				} else {
					assert.Nil(t, capture.resourceOwner)
				}

				// use is recorded
				stored = tester.Fetch(&Key{}, key.ID()).(*Key)
				assert.NotNil(t, stored.Used)
			})
		}
	})
}
