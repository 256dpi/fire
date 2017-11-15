package flame

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/spec"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

var testSecret = "abcd1234abcd1234"
var testPassword = "foo"

func TestIntegration(t *testing.T) {
	cleanSubStore()

	var allowedScope = oauth2.ParseScope("foo bar")
	var requiredScope = oauth2.ParseScope("foo")

	p := DefaultPolicy(testSecret)

	p.PasswordGrant = true
	p.ClientCredentialsGrant = true
	p.ImplicitGrant = true

	p.GrantStrategy = func(req *GrantRequest) (oauth2.Scope, error) {
		if !allowedScope.Includes(req.Scope) {
			return nil, ErrInvalidScope
		}

		if !req.Scope.Includes(requiredScope) {
			return nil, ErrInvalidScope
		}

		return req.Scope, nil
	}

	manager := NewAuthenticator(testStore, p)
	manager.Reporter = func(err error) {
		t.Error(err)
	}

	app1 := saveModel(&Application{
		Name:        "Application 1",
		Key:         "app1",
		SecretHash:  mustHash(testPassword),
		RedirectURI: "http://example.com/callback1",
	}).(*Application)

	app2 := saveModel(&Application{
		Name:        "Application 2",
		Key:         "app2",
		SecretHash:  mustHash(testPassword),
		RedirectURI: "http://example.com/callback2",
	}).(*Application)

	user := saveModel(&User{
		Name:         "User",
		Email:        "user@example.com",
		PasswordHash: mustHash(testPassword),
	}).(*User)

	config := spec.Default(newHandler(manager, true))

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

	config.ExpectedExpiresIn = int(manager.policy.AccessTokenLifespan / time.Second)

	expiredToken := saveModel(&AccessToken{
		ExpiresAt: time.Now().Add(-manager.policy.AccessTokenLifespan),
		Scope:     []string{"foo"},
		ClientID:  app1.ID(),
	}).(*AccessToken)

	insufficientToken := saveModel(&AccessToken{
		ExpiresAt: time.Now().Add(manager.policy.AccessTokenLifespan),
		Scope:     []string{},
		ClientID:  app1.ID(),
	}).(*AccessToken)

	config.UnknownToken = mustGenerateAccessToken(p, bson.NewObjectId(), time.Now())
	config.ExpiredToken = mustGenerateAccessToken(p, expiredToken.ID(), expiredToken.ExpiresAt)
	config.InsufficientToken = mustGenerateAccessToken(p, insufficientToken.ID(), insufficientToken.ExpiresAt)

	config.PrimaryRedirectURI = "http://example.com/callback1"
	config.SecondaryRedirectURI = "http://example.com/callback2"

	validRefreshToken := saveModel(&RefreshToken{
		ExpiresAt: time.Now().Add(manager.policy.RefreshTokenLifespan),
		Scope:     []string{"foo", "bar"},
		ClientID:  app1.ID(),
	}).(*RefreshToken)

	expiredRefreshToken := saveModel(&RefreshToken{
		ExpiresAt: time.Now().Add(-manager.policy.RefreshTokenLifespan),
		Scope:     []string{"foo", "bar"},
		ClientID:  app1.ID(),
	}).(*RefreshToken)

	config.UnknownRefreshToken = mustGenerateRefreshToken(p, bson.NewObjectId(), time.Now())
	config.ValidRefreshToken = mustGenerateRefreshToken(p, validRefreshToken.ID(), validRefreshToken.ExpiresAt)
	config.ExpiredRefreshToken = mustGenerateRefreshToken(p, expiredRefreshToken.ID(), expiredRefreshToken.ExpiresAt)

	config.AuthorizationParams = map[string]string{
		"username": user.Email,
		"password": testPassword,
	}

	spec.Run(t, config)
}

func TestPublicAccess(t *testing.T) {
	manager := NewAuthenticator(testStore, DefaultPolicy(testSecret))
	handler := newHandler(manager, false)

	testRequest(handler, "GET", "/api/protected", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, "OK", r.Body.String())
	})
}

func mustGenerateAccessToken(p *Policy, id bson.ObjectId, expiresAt time.Time) string {
	str, err := p.GenerateToken(id, time.Now(), expiresAt, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}

func mustGenerateRefreshToken(p *Policy, id bson.ObjectId, expiresAt time.Time) string {
	str, err := p.GenerateToken(id, time.Now(), expiresAt, nil, nil)
	if err != nil {
		panic(err)
	}

	return str
}
