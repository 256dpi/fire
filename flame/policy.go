package flame

import (
	"errors"
	"net/http"
	"time"

	"github.com/256dpi/oauth2"
	"github.com/dgrijalva/jwt-go"
	"github.com/globalsign/mgo/bson"
)

// ErrInvalidFilter should be returned by the Filter to indicate that the request
// includes invalid filter parameters.
var ErrInvalidFilter = errors.New("invalid filter")

// ErrGrantRejected should be returned by the GrantStrategy to indicate a rejection
// of the grant based on the provided conditions.
var ErrGrantRejected = errors.New("grant rejected")

// ErrInvalidScope should be returned by the GrantStrategy to indicate that the
// requested scope exceeds the grantable scope.
var ErrInvalidScope = errors.New("invalid scope")

// A Policy configures the provided authentication schemes.
type Policy struct {
	// The secret should be at least 16 characters long.
	Secret []byte

	// The available grants.
	PasswordGrant          bool
	ClientCredentialsGrant bool
	ImplicitGrant          bool

	// The token models.
	AccessToken  Token
	RefreshToken Token

	// The client models.
	Clients []Client

	// ResourceOwners should return a list of resource owner models that are
	// tried in order to resolve grant requests.
	ResourceOwners func(Client) []ResourceOwner

	// Filter should return a filter that should be applied when looking up a
	// resource owner. This callback can be used to select resource owners
	// based on other request parameters. It can return ErrInvalidFilter to
	// cancel the authentication request.
	Filter func(ResourceOwner, *http.Request) (bson.M, error)

	// GrantStrategy is invoked by the authenticator with the grant type, the
	// requested scope, the client and the resource owner before issuing an
	// access token. The callback should return no error and the scope that
	// should be granted. It can return ErrGrantRejected or ErrInvalidScope to
	// cancel the grant request.
	//
	// Note: ResourceOwner is not set for a client credentials grant.
	GrantStrategy func(oauth2.Scope, Client, ResourceOwner) (oauth2.Scope, error)

	// TokenData should return a map of data that should be included in the JWT
	// tokens under the "dat" field.
	TokenData func(Client, ResourceOwner, Token) map[string]interface{}

	// The token used lifespans.
	AccessTokenLifespan  time.Duration
	RefreshTokenLifespan time.Duration
}

// TokenClaims represents the data included in an access token and refresh token.
type TokenClaims struct {
	jwt.StandardClaims

	// Data contains user defined key value pairs.
	Data map[string]interface{} `json:"dat"`
}

// DefaultGrantStrategy grants only empty scopes.
func DefaultGrantStrategy(scope oauth2.Scope, _ Client, _ ResourceOwner) (oauth2.Scope, error) {
	// check scope
	if !scope.Empty() {
		return nil, ErrInvalidScope
	}

	return scope, nil
}

// DefaultTokenData adds the user's id to the token data claim.
func DefaultTokenData(_ Client, ro ResourceOwner, _ Token) map[string]interface{} {
	if ro != nil {
		return map[string]interface{}{
			"user": ro.ID(),
		}
	}

	return nil
}

// DefaultPolicy returns a simple policy that uses all built-in models and
// strategies.
//
// Note: The secret should be at least 16 characters long.
func DefaultPolicy(secret string) *Policy {
	return &Policy{
		Secret:       []byte(secret),
		AccessToken:  &AccessToken{},
		RefreshToken: &RefreshToken{},
		Clients:      []Client{&Application{}},
		ResourceOwners: func(_ Client) []ResourceOwner {
			return []ResourceOwner{&User{}}
		},
		GrantStrategy:        DefaultGrantStrategy,
		TokenData:            DefaultTokenData,
		AccessTokenLifespan:  time.Hour,
		RefreshTokenLifespan: 7 * 24 * time.Hour,
	}
}

// GenerateToken returns a new token for the provided information.
func (p *Policy) GenerateToken(id bson.ObjectId, issuedAt, expiresAt time.Time, client Client, resourceOwner ResourceOwner, token Token) (string, error) {
	// prepare claims
	claims := &TokenClaims{}
	claims.Id = id.Hex()
	claims.IssuedAt = issuedAt.Unix()
	claims.ExpiresAt = expiresAt.Unix()

	// set user data
	if p.TokenData != nil {
		claims.Data = p.TokenData(client, resourceOwner, token)
	}

	// create token
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// sign token
	str, err := tkn.SignedString(p.Secret)
	if err != nil {
		return "", nil
	}

	return str, nil
}

// ParseToken will parse the presented token and return its claims, if it is
// expired and eventual errors.
func (p *Policy) ParseToken(str string) (*TokenClaims, bool, error) {
	// parse token and check id
	var claims TokenClaims
	_, err := jwt.ParseWithClaims(str, &claims, func(_ *jwt.Token) (interface{}, error) {
		return p.Secret, nil
	})
	if valErr, ok := err.(*jwt.ValidationError); ok && valErr.Errors == jwt.ValidationErrorExpired {
		return nil, true, err
	} else if err != nil {
		return nil, false, err
	} else if !bson.IsObjectIdHex(claims.Id) {
		return nil, false, errors.New("invalid id")
	}

	return &claims, false, nil
}
