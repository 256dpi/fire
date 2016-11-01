package auth

import (
	"time"

	"github.com/gonfire/oauth2/hmacsha"
)

// A GrantRequest is used in conjunction with the GrantStrategy.
type GrantRequest struct {
	Scope         []string
	Client        Client
	ResourceOwner ResourceOwner
}

// The GrantStrategy is invoked by the authenticator with the grant type, the
// requested scope, the client and the resource owner before issuing an access
// token. The callback should return the scopes that should be granted.
//
// Note: The Owner is not set for a client credentials grant.
type GrantStrategy func(req *GrantRequest) (bool, []string)

// DefaultGrantStrategy grants the complete requested scope.
func DefaultGrantStrategy(req *GrantRequest) (bool, []string) {
	return true, req.Scope
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
	GrantStrategy GrantStrategy

	AccessTokenLifespan  time.Duration
	RefreshTokenLifespan time.Duration
}

// DefaultPolicy returns a simple policy that uses all built-in models and
// strategies.
func DefaultPolicy(secret string) *Policy {
	return &Policy{
		Secret:               []byte(secret),
		AccessToken:          &AccessToken{},
		RefreshToken:         &RefreshToken{},
		Client:               &Application{},
		ResourceOwner:        &User{},
		GrantStrategy:        DefaultGrantStrategy,
		AccessTokenLifespan:  time.Hour,
		RefreshTokenLifespan: 7 * 24 * time.Hour,
	}
}

// NewKeyAndSignature returns a new key with a matching signature that can be
// used to issue custom access tokens.
func (p *Policy) NewKeyAndSignature() (string, string, error) {
	token, err := hmacsha.Generate(p.Secret, 32)
	if err != nil {
		// TODO: Report error.
		return "", "", err
	}

	return token.String(), token.SignatureString(), nil
}
