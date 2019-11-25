package flame

import (
	"errors"
	"net/http"
	"time"

	"github.com/256dpi/oauth2"
	"github.com/dgrijalva/jwt-go"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
)

// ErrInvalidFilter should be returned by the ResourceOwnerFilter to indicate
// that the request includes invalid filter parameters.
var ErrInvalidFilter = errors.New("invalid filter")

// ErrGrantRejected should be returned by the GrantStrategy to indicate a rejection
// of the grant based on the provided conditions.
var ErrGrantRejected = errors.New("grant rejected")

// ErrApprovalRejected should be returned by the ApproveStrategy to indicate a
// rejection of the approval based on the provided conditions.
var ErrApprovalRejected = errors.New("approval rejected")

// ErrInvalidScope should be returned by the GrantStrategy to indicate that the
// requested scope exceeds the grantable scope.
var ErrInvalidScope = errors.New("invalid scope")

// Policy configures the provided authentication and authorization schemes used
// by the authenticator.
type Policy struct {
	// The secret used to sign and verify all tokens. Should be at least 16
	// characters long to ensure strong security.
	Secret string

	// The available authentication and authorization grants.
	PasswordGrant          bool
	ClientCredentialsGrant bool
	ImplicitGrant          bool
	AuthorizationCodeGrant bool

	// The token model.
	Token GenericToken

	// The client models.
	Clients []Client

	// ClientFilter may return a filter that should be applied when looking
	// up a client. This callback can be used to select clients based on other
	// request parameters. It can return ErrInvalidFilter to cancel the
	// authentication request.
	ClientFilter func(Client, *http.Request) (bson.M, error)

	// ResourceOwners should return a list of resource owner models that are
	// tried in order to resolve grant requests.
	ResourceOwners func(Client) []ResourceOwner

	// ResourceOwnerFilter may return a filter that should be applied when
	// looking up a resource owner. This callback can be used to select resource
	// owners based on other request parameters. It can return ErrInvalidFilter
	// to cancel the authentication request.
	ResourceOwnerFilter func(ResourceOwner, *http.Request) (bson.M, error)

	// GrantStrategy is invoked by the authenticator with the requested scope,
	// the client and the resource owner before issuing an access token. The
	// callback should return the scope that should be granted. It can return
	// ErrGrantRejected or ErrInvalidScope to cancel the grant request.
	//
	// Note: ResourceOwner is not set for a client credentials grant.
	GrantStrategy func(oauth2.Scope, Client, ResourceOwner) (oauth2.Scope, error)

	// The URL to the page that obtains the approval of the user in implicit and
	// authorization code grants.
	ApprovalURL func(Client) (string, error)

	// ApproveStrategy is invoked by the authenticator to verify the
	// authorization approval by an authenticated resource owner in the implicit
	// grant and authorization code grant flows. The callback should return the
	// scope that should be granted. It may return ErrApprovalRejected or
	// ErrInvalidScope to cancel the approval request.
	//
	// Note: GenericToken represents the token that authorizes the resource
	// owner to give the approval.
	ApproveStrategy func(GenericToken, oauth2.Scope, Client, ResourceOwner) (oauth2.Scope, error)

	// TokenData may return a map of data that should be included in the
	// generated JWT tokens as the "dat" field.
	TokenData func(Client, ResourceOwner, GenericToken) map[string]interface{}

	// The token and code lifespans.
	AccessTokenLifespan       time.Duration
	RefreshTokenLifespan      time.Duration
	AuthorizationCodeLifespan time.Duration
}

// DefaultGrantStrategy grants only empty scopes.
func DefaultGrantStrategy(scope oauth2.Scope, _ Client, _ ResourceOwner) (oauth2.Scope, error) {
	// check scope
	if !scope.Empty() {
		return nil, ErrInvalidScope
	}

	return scope, nil
}

// StaticApprovalURL returns a static approval URL.
func StaticApprovalURL(url string) func(Client) (string, error) {
	return func(Client) (string, error) {
		return url, nil
	}
}

// DefaultApproveStrategy rejects all approvals.
func DefaultApproveStrategy(GenericToken, oauth2.Scope, Client, ResourceOwner) (oauth2.Scope, error) {
	return nil, ErrApprovalRejected
}

// DefaultTokenData adds the user's id to the token data claim.
func DefaultTokenData(_ Client, ro ResourceOwner, _ GenericToken) map[string]interface{} {
	if ro != nil {
		return map[string]interface{}{
			"user": ro.ID(),
		}
	}

	return nil
}

// DefaultPolicy returns a simple policy that uses all built-in models and
// strategies.
func DefaultPolicy(secret string) *Policy {
	return &Policy{
		Secret:  secret,
		Token:   &Token{},
		Clients: []Client{&Application{}},
		ResourceOwners: func(_ Client) []ResourceOwner {
			return []ResourceOwner{&User{}}
		},
		GrantStrategy:             DefaultGrantStrategy,
		ApprovalURL:               StaticApprovalURL(""),
		ApproveStrategy:           DefaultApproveStrategy,
		TokenData:                 DefaultTokenData,
		AccessTokenLifespan:       time.Hour,
		RefreshTokenLifespan:      7 * 24 * time.Hour,
		AuthorizationCodeLifespan: time.Minute,
	}
}

// GenerateJWT returns a new JWT token for the provided information.
func (p *Policy) GenerateJWT(token GenericToken, client Client, resourceOwner ResourceOwner) (string, error) {
	// get data
	data := token.GetTokenData()

	// prepare claims
	claims := Claims{}
	claims.Id = token.ID().Hex()
	claims.IssuedAt = token.ID().Timestamp().Unix()
	claims.ExpiresAt = data.ExpiresAt.Unix()

	// set user data
	if p.TokenData != nil {
		claims.Data = p.TokenData(client, resourceOwner, token)
	}

	// create token
	str, err := GenerateJWT(p.Secret, claims)
	if err != nil {
		return "", nil
	}

	return str, nil
}

// ParseJWT will parse the presented token and return its claims, if it is
// expired and eventual errors.
func (p *Policy) ParseJWT(str string) (*Claims, bool, error) {
	// parse token and check expired errors
	_, claims, err := ParseJWT(p.Secret, str)
	if valErr, ok := err.(*jwt.ValidationError); ok && valErr.Errors == jwt.ValidationErrorExpired {
		return nil, true, err
	} else if err != nil {
		return nil, false, err
	}

	// parse id
	_, err = coal.FromHex(claims.Id)
	if err != nil {
		return nil, false, errors.New("invalid id")
	}

	return claims, false, nil
}
