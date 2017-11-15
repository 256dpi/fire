package flame

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/spec"
	"github.com/dgrijalva/jwt-go"
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

	config.UnknownToken = mustGenerateAccessToken(bson.NewObjectId(), p.Secret, time.Now(), nil)
	config.ExpiredToken = mustGenerateAccessToken(expiredToken.ID(), p.Secret, expiredToken.ExpiresAt, nil)
	config.InsufficientToken = mustGenerateAccessToken(insufficientToken.ID(), p.Secret, insufficientToken.ExpiresAt, nil)

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

	config.UnknownRefreshToken = mustGenerateRefreshToken(bson.NewObjectId(), p.Secret, time.Now())
	config.ValidRefreshToken = mustGenerateRefreshToken(validRefreshToken.ID(), p.Secret, validRefreshToken.ExpiresAt)
	config.ExpiredRefreshToken = mustGenerateRefreshToken(expiredRefreshToken.ID(), p.Secret, expiredRefreshToken.ExpiresAt)

	config.AuthorizationParams = map[string]string{
		"username": user.Email,
		"password": testPassword,
	}

	spec.Run(t, config)
}

func TestJWTToken(t *testing.T) {
	id := bson.NewObjectId()
	tt := time.Now()
	sig, err := generateAccessToken(id, []byte("abcd"), tt, tt, &User{Name: "Hello"})
	assert.NoError(t, err)

	var claims accessTokenClaims
	token, err := jwt.ParseWithClaims(sig, &claims, func(t *jwt.Token) (interface{}, error) {
		return []byte("abcd"), nil
	})
	assert.NoError(t, err)
	assert.NotNil(t, token)

	assert.Equal(t, id.Hex(), claims.Id)
	assert.Equal(t, tt.Unix(), claims.IssuedAt)
	assert.Equal(t, tt.Unix(), claims.ExpiresAt)
	assert.Equal(t, map[string]interface{}{
		"name": "Hello",
	}, claims.Data)
}

func TestPublicAccess(t *testing.T) {
	manager := NewAuthenticator(testStore, DefaultPolicy(testSecret))
	handler := newHandler(manager, false)

	testRequest(handler, "GET", "/api/protected", nil, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, "OK", r.Body.String())
	})
}

func mustGenerateAccessToken(id bson.ObjectId, secret []byte, expiresAt time.Time, ro ResourceOwner) string {
	str, err := generateAccessToken(id, secret, time.Now(), expiresAt, ro)
	if err != nil {
		panic(err)
	}

	return str
}

func mustGenerateRefreshToken(id bson.ObjectId, secret []byte, expiresAt time.Time) string {
	str, err := generateRefreshToken(id, secret, time.Now(), expiresAt)
	if err != nil {
		panic(err)
	}

	return str
}
