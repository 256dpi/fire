package auth

import (
	"testing"
	"time"

	"github.com/gonfire/fire"
	"github.com/gonfire/oauth2/hmacsha"
	"github.com/gonfire/oauth2/spec"
	"github.com/stretchr/testify/assert"
)

var testSecret = []byte("abcd1234abcd1234")

func TestIntegration(t *testing.T) {
	p := DefaultPolicy(testSecret)
	p.PasswordGrant = true

	auth := New(getCleanStore(), p, "auth")

	app1 := saveModel(&Application{
		Name:         "Application 1",
		Key:          "app1",
		SecretHash:   mustHash("foo"),
		Scope:        "foo",
		RedirectURIs: []string{"http://example.com/callback1"},
	})

	saveModel(&Application{
		Name:         "Application 2",
		Key:          "app2",
		SecretHash:   mustHash("foo"),
		Scope:        "foo",
		RedirectURIs: []string{"http://example.com/callback2"},
	})

	saveModel(&User{
		Name:         "User",
		Email:        "user@example.com",
		PasswordHash: mustHash("foo"),
	})

	unknownToken := hmacsha.MustGenerate(testSecret, 32)
	expiredToken := hmacsha.MustGenerate(testSecret, 32)
	insufficientToken := hmacsha.MustGenerate(testSecret, 32)

	saveModel(&Credential{
		Signature: expiredToken.SignatureString(),
		ExpiresAt: time.Now().Add(-auth.Policy.AccessTokenLifespan),
		Scope:     "foo",
		ClientID:  app1.ID(),
	})

	saveModel(&Credential{
		Signature: insufficientToken.SignatureString(),
		ExpiresAt: time.Now().Add(auth.Policy.AccessTokenLifespan),
		Scope:     "",
		ClientID:  app1.ID(),
	})

	unknownRefreshToken := hmacsha.MustGenerate(testSecret, 32)
	validRefreshToken := hmacsha.MustGenerate(testSecret, 32)
	expiredRefreshToken := hmacsha.MustGenerate(testSecret, 32)

	saveModel(&Credential{
		Signature: validRefreshToken.SignatureString(),
		ExpiresAt: time.Now().Add(auth.Policy.AccessTokenLifespan),
		Scope:     "foo",
		ClientID:  app1.ID(),
	})

	saveModel(&Credential{
		Signature: expiredRefreshToken.SignatureString(),
		ExpiresAt: time.Now().Add(-auth.Policy.AccessTokenLifespan),
		Scope:     "foo",
		ClientID:  app1.ID(),
	})

	config := spec.Default(newHandler(auth))

	config.PasswordGrantSupport = true
	config.ClientCredentialsGrantSupport = true
	config.ImplicitGrantSupport = true
	config.RefreshTokenGrantSupport = true

	config.PrimaryClientID = "app1"
	config.PrimaryClientSecret = "foo"
	config.SecondaryClientID = "app2"
	config.SecondaryClientSecret = "foo"

	config.ResourceOwnerUsername = "user1"
	config.ResourceOwnerPassword = "foo"

	config.InvalidScope = "baz"
	config.ValidScope = "foo bar"
	config.ExceedingScope = "foo bar baz"

	config.ExpectedExpireIn = int(auth.Policy.AccessTokenLifespan / time.Second)

	config.InvalidToken = "invalid"
	config.UnknownToken = unknownToken.String()
	config.ExpiredToken = expiredToken.String()
	config.InsufficientToken = insufficientToken.String()

	config.InvalidRedirectURI = "http://invalid.com"
	config.PrimaryRedirectURI = "http://example.com/callback1"
	config.SecondaryRedirectURI = "http://example.com/callback2"

	config.InvalidRefreshToken = "invalid"
	config.UnknownRefreshToken = unknownRefreshToken.String()
	config.ValidRefreshToken = validRefreshToken.String()
	config.ExpiredRefreshToken = expiredRefreshToken.String()

	config.AuthorizationParams = map[string]string{
		"username": "user@example.com",
		"password": "foo",
	}

	spec.Run(t, config)
}

func TestAuthenticatorInspect(t *testing.T) {
	p := DefaultPolicy(testSecret)
	p.PasswordGrant = true

	a := New(getCleanStore(), p, "auth")

	assert.Equal(t, fire.ComponentInfo{
		Name: "Authenticator",
		Settings: fire.Map{
			"Prefix":                         "auth",
			"Allow Password Grant":           "true",
			"Allow Client Credentials Grant": "false",
			"Allow Implicit Grant":           "false",
			"Access Token Lifespan":          "1h0m0s",
			"Refresh Token Lifespan":         "168h0m0s",
			"Access Token Model":             "auth.Credential",
			"Refresh Token Model":            "auth.Credential",
			"Client Model":                   "auth.Application",
			"Resource Owner Model":           "auth.User",
		},
	}, a.Describe())
}
