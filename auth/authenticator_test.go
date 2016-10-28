package auth

import (
	"testing"
	"time"

	"github.com/gonfire/oauth2"
	"github.com/gonfire/oauth2/hmacsha"
	"github.com/gonfire/oauth2/spec"
)

var testSecret = "abcd1234abcd1234"

func TestIntegration(t *testing.T) {
	var allowedScope = oauth2.ParseScope("foo bar")
	var requiredScope = oauth2.ParseScope("foo")

	p := DefaultPolicy(testSecret)

	p.PasswordGrant = true
	p.ClientCredentialsGrant = true
	p.ImplicitGrant = true

	p.GrantStrategy = func(req *GrantRequest) (bool, []string) {
		if !allowedScope.Includes(req.Scope) {
			return false, []string{}
		}

		if !oauth2.Scope(req.Scope).Includes(requiredScope) {
			return false, []string{}
		}

		return true, []string(req.Scope)
	}

	auth := New(getCleanStore(), p)

	app1 := saveModel(&Application{
		Name:        "Application 1",
		Key:         "app1",
		SecretHash:  mustHash("foo"),
		Scope:       "foo",
		RedirectURI: "http://example.com/callback1",
	})

	saveModel(&Application{
		Name:        "Application 2",
		Key:         "app2",
		SecretHash:  mustHash("foo"),
		Scope:       "foo",
		RedirectURI: "http://example.com/callback2",
	})

	saveModel(&User{
		Name:         "User",
		Email:        "user@example.com",
		PasswordHash: mustHash("foo"),
	})

	unknownToken := hmacsha.MustGenerate(p.Secret, 32)
	expiredToken := hmacsha.MustGenerate(p.Secret, 32)
	insufficientToken := hmacsha.MustGenerate(p.Secret, 32)

	saveModel(&AccessToken{
		Signature: expiredToken.SignatureString(),
		ExpiresAt: time.Now().Add(-auth.policy.AccessTokenLifespan),
		Scope:     []string{"foo"},
		ClientID:  app1.ID(),
	})

	saveModel(&AccessToken{
		Signature: insufficientToken.SignatureString(),
		ExpiresAt: time.Now().Add(auth.policy.AccessTokenLifespan),
		Scope:     []string{},
		ClientID:  app1.ID(),
	})

	unknownRefreshToken := hmacsha.MustGenerate(p.Secret, 32)
	validRefreshToken := hmacsha.MustGenerate(p.Secret, 32)
	expiredRefreshToken := hmacsha.MustGenerate(p.Secret, 32)

	saveModel(&RefreshToken{
		Signature: validRefreshToken.SignatureString(),
		ExpiresAt: time.Now().Add(auth.policy.RefreshTokenLifespan),
		Scope:     []string{"foo", "bar"},
		ClientID:  app1.ID(),
	})

	saveModel(&RefreshToken{
		Signature: expiredRefreshToken.SignatureString(),
		ExpiresAt: time.Now().Add(-auth.policy.RefreshTokenLifespan),
		Scope:     []string{"foo", "bar"},
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

	config.ResourceOwnerUsername = "user@example.com"
	config.ResourceOwnerPassword = "foo"

	config.InvalidScope = "baz"
	config.ValidScope = "foo bar"
	config.ExceedingScope = "foo bar baz"

	config.ExpectedExpiresIn = int(auth.policy.AccessTokenLifespan / time.Second)

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
