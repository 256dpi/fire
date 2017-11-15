package flame

import (
	"errors"
	"time"

	"github.com/256dpi/oauth2"
	"gopkg.in/mgo.v2/bson"
)

// A GrantRequest is used in conjunction with the GrantStrategy.
type GrantRequest struct {
	// The scope that has been requested.
	Scope oauth2.Scope

	// The client that made the access request.
	Client Client

	// The resource owner that gave his consent.
	//
	// Note: ResourceOwner is not set for a client credentials grant.
	ResourceOwner ResourceOwner
}

// ErrGrantRejected should be returned by the GrantStrategy to indicate a rejection
// of the grant based on the provided conditions.
var ErrGrantRejected = errors.New("grant rejected")

// ErrInvalidScope should be returned by the GrantStrategy to indicate that the
// requested scope exceeds the grantable scope.
var ErrInvalidScope = errors.New("invalid scope")

// The GrantStrategy is invoked by the manager with the grant type, the
// requested scope, the client and the resource owner before issuing an access
// token. The callback should return no error and the scope that should be granted.
// It can return ErrGrantRejected or ErrInvalidScope to cancel the grant request.
type GrantStrategy func(req *GrantRequest) (oauth2.Scope, error)

// DefaultGrantStrategy grants the requested scope.
func DefaultGrantStrategy(req *GrantRequest) (oauth2.Scope, error) {
	return req.Scope, nil
}

// A Policy configures the provided authentication schemes.
type Policy struct {
	// The shared secret which should be at least 16 characters.
	Secret []byte

	// The available grants.
	PasswordGrant          bool
	ClientCredentialsGrant bool
	ImplicitGrant          bool

	// The used models and strategies.
	AccessToken    Token
	RefreshToken   Token
	Clients        []Client
	ResourceOwners []ResourceOwner
	GrantStrategy  GrantStrategy

	// The token used lifespans.
	AccessTokenLifespan  time.Duration
	RefreshTokenLifespan time.Duration

	// The optional automated cleanup of expires tokens.
	AutomatedCleanup bool
}

// DefaultPolicy returns a simple policy that uses all built-in models and
// strategies.
func DefaultPolicy(secret string) *Policy {
	return &Policy{
		Secret:               []byte(secret),
		AccessToken:          &AccessToken{},
		RefreshToken:         &RefreshToken{},
		Clients:              []Client{&Application{}},
		ResourceOwners:       []ResourceOwner{&User{}},
		GrantStrategy:        DefaultGrantStrategy,
		AccessTokenLifespan:  time.Hour,
		RefreshTokenLifespan: 7 * 24 * time.Hour,
		AutomatedCleanup:     true,
	}
}

// NewAccessToken returns a new access token for the provided information.
func (p *Policy) NewAccessToken(id bson.ObjectId, issuedAt, expiresAt time.Time, ro ResourceOwner) (string, error) {
	str, err := generateAccessToken(id, p.Secret, issuedAt, expiresAt, ro)
	if err != nil {
		return "", err
	}

	return str, nil
}
