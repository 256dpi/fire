package auth

import (
	"time"

	"github.com/gonfire/oauth2"
	"golang.org/x/crypto/bcrypt"
)

// A GrantRequest is used in conjunction with the GrantStrategy.
type GrantRequest struct {
	Scope         oauth2.Scope
	Client        Client
	ResourceOwner ResourceOwner
}

// The GrantStrategy is invoked by the authenticator with the grant type, the
// requested scope, the client and the resource owner before issuing an access
// token. The callback should return the scopes that should be granted.
//
// Note: The Owner is not set for a client credentials grant.
type GrantStrategy func(req *GrantRequest) (bool, oauth2.Scope)

// DefaultGrantStrategy grants the complete requested scope.
func DefaultGrantStrategy(req *GrantRequest) (bool, oauth2.Scope) {
	return true, req.Scope
}

// The CompareStrategy is invoked by the authenticator with the stored password
// hash and submitted password of a resource owner. The callback is responsible
// for comparing the submitted password with the stored hash and should return an
// error if they do not match.
type CompareStrategy func(hash, password []byte) error

// DefaultCompareStrategy uses bcrypt to compare the hash and the password.
func DefaultCompareStrategy(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

// A Policy configures the provided authentication schemes.
type Policy struct {
	Secret []byte

	PasswordGrant          bool
	ClientCredentialsGrant bool
	ImplicitGrant          bool

	AccessToken   Token
	RefreshToken  Token
	Client        Client
	ResourceOwner ResourceOwner

	GrantStrategy   GrantStrategy
	CompareStrategy CompareStrategy

	AccessTokenLifespan  time.Duration
	RefreshTokenLifespan time.Duration
}

// DefaultPolicy returns a simple policy that uses all built-in models and
// strategies.
func DefaultPolicy(secret string) *Policy {
	return &Policy{
		Secret:               []byte(secret),
		AccessToken:          &Credential{},
		RefreshToken:         &Credential{},
		Client:               &Application{},
		ResourceOwner:        &User{},
		GrantStrategy:        DefaultGrantStrategy,
		CompareStrategy:      DefaultCompareStrategy,
		AccessTokenLifespan:  time.Hour,
		RefreshTokenLifespan: 7 * 24 * time.Hour,
	}
}
