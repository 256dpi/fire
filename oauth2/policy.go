package oauth2

import (
	"time"

	"github.com/gonfire/fire/model"
	"golang.org/x/crypto/bcrypt"
)

// A GrantRequest is used in conjunction with the GrantStrategy.
type GrantRequest struct {
	GrantType       string
	RequestedScopes []string
	Client          model.Model
	Owner           model.Model
}

// The GrantStrategy is invoked by the Authenticator with the grant type,
// requested scopes, the client and the owner before issuing an AccessToken.
// The callback should return a list of additional scopes that should be granted.
//
// Note: The Owner is not set for a client credentials grant.
type GrantStrategy func(req *GrantRequest) []string

// DefaultGrantStrategy grants all requested scopes.
func DefaultGrantStrategy(req *GrantRequest) []string {
	return req.RequestedScopes
}

// The CompareStrategy is invoked by the Authenticator with the stored password
// hash and submitted password of a owner. The callback is responsible for
// comparing the submitted password with the stored hash and should return an
// error if they do not match.
type CompareStrategy func(hash, password []byte) error

// DefaultCompareStrategy uses bcrypt to compare the hash and the password.
func DefaultCompareStrategy(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

// A Policy is used to prepare an authentication policy for an Authenticator.
type Policy struct {
	PasswordGrant          bool
	ClientCredentialsGrant bool
	ImplicitGrant          bool

	Secret []byte

	OwnerModel       OwnerModel
	ClientModel      ClientModel
	AccessTokenModel AccessTokenModel

	GrantStrategy   GrantStrategy
	CompareStrategy CompareStrategy
	TokenLifespan   time.Duration
}

// DefaultPolicy returns a simple policy that provides a starting point.
func DefaultPolicy() *Policy {
	return &Policy{
		OwnerModel:       &User{},
		ClientModel:      &Application{},
		AccessTokenModel: &AccessToken{},
		GrantStrategy:    DefaultGrantStrategy,
		CompareStrategy:  DefaultCompareStrategy,
		TokenLifespan:    time.Hour,
	}
}
