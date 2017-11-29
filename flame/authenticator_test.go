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

func TestIntegration(t *testing.T) {
	tester.Clean()

	var testPassword = "foo"
	var allowedScope = oauth2.ParseScope("foo bar")
	var requiredScope = oauth2.ParseScope("foo")

	p := DefaultPolicy("")

	p.PasswordGrant = true
	p.ClientCredentialsGrant = true
	p.ImplicitGrant = true

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

	config.AuthorizationParams = map[string]string{
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
