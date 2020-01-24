package flame

import (
	"errors"
	"net/http"
	"time"

	"github.com/256dpi/oauth2/v2"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

// ErrInvalidFilter should be returned by the ResourceOwnerFilter to indicate
// that the request includes invalid filter parameters.
var ErrInvalidFilter = errors.New("invalid filter")

// ErrInvalidRedirectURI should be returned by the RedirectURIValidator to
// indicate that the redirect URI is invalid.
var ErrInvalidRedirectURI = errors.New("invalid redirect uri")

// ErrGrantRejected should be returned by the GrantStrategy to indicate a rejection
// of the grant based on the provided conditions.
var ErrGrantRejected = errors.New("grant rejected")

// ErrApprovalRejected should be returned by the ApproveStrategy to indicate a
// rejection of the approval based on the provided conditions.
var ErrApprovalRejected = errors.New("approval rejected")

// ErrInvalidScope should be returned by the GrantStrategy to indicate that the
// requested scope exceeds the grantable scope.
var ErrInvalidScope = errors.New("invalid scope")

// Grants defines the selected grants.
type Grants struct {
	Password          bool
	ClientCredentials bool
	Implicit          bool
	AuthorizationCode bool
	RefreshToken      bool
}

// Policy configures the provided authentication and authorization schemes used
// by the authenticator.
type Policy struct {
	// The secret used to sign and verify all tokens. Should be at least 16
	// characters long to ensure strong security.
	Secret []byte

	// The token model.
	Token GenericToken

	// The client models.
	Clients []Client

	// Grants should return the permitted grants for the provided client.
	Grants func(Client) (Grants, error)

	// ClientFilter may return a filter that should be applied when looking
	// up a client. This callback can be used to select clients based on other
	// request parameters. It can return ErrInvalidFilter to cancel the
	// authentication request.
	ClientFilter func(Client, *http.Request) (bson.M, error)

	// RedirectURIValidator should validate a redirect URI and return the valid
	// or corrected redirect URI. It can return ErrInvalidRedirectURI to to
	// cancel the authorization request. The validator is during the
	// authorization and the token request. If the result differs, no token will
	// be issue and the request aborted.
	RedirectURIValidator func(Client, string) (string, error)

	// ResourceOwners should return a list of resource owner models that are
	// tried in order to resolve grant requests.
	ResourceOwners func(Client) ([]ResourceOwner, error)

	// ResourceOwnerFilter may return a filter that should be applied when
	// looking up a resource owner. This callback can be used to select resource
	// owners based on other request parameters. It can return ErrInvalidFilter
	// to cancel the authentication request.
	ResourceOwnerFilter func(Client, ResourceOwner, *http.Request) (bson.M, error)

	// GrantStrategy is invoked by the authenticator with the requested scope,
	// the client and the resource owner before issuing an access token. The
	// callback should return the scope that should be granted. It can return
	// ErrGrantRejected or ErrInvalidScope to cancel the grant request.
	//
	// Note: ResourceOwner is not set for a client credentials grant.
	GrantStrategy func(Client, ResourceOwner, oauth2.Scope) (oauth2.Scope, error)

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
	ApproveStrategy func(Client, ResourceOwner, GenericToken, oauth2.Scope) (oauth2.Scope, error)

	// TokenData may return a map of data that should be included in the
	// generated JWT tokens as the "dat" field as well as in the token
	// introspection's response "extra" field.
	TokenData func(Client, ResourceOwner, GenericToken) map[string]interface{}

	// The token and code lifespans.
	AccessTokenLifespan       time.Duration
	RefreshTokenLifespan      time.Duration
	AuthorizationCodeLifespan time.Duration
}

// StaticGrants always selects the specified grants.
func StaticGrants(password, clientCredentials, implicit, authorizationCode, refreshToken bool) func(Client) (Grants, error) {
	return func(Client) (Grants, error) {
		return Grants{
			Password:          password,
			ClientCredentials: clientCredentials,
			Implicit:          implicit,
			AuthorizationCode: authorizationCode,
			RefreshToken:      refreshToken,
		}, nil
	}
}

// DefaultRedirectURIValidator will check the redirect URI against the client
// model using the ValidRedirectURI method.
func DefaultRedirectURIValidator(client Client, uri string) (string, error) {
	// check model
	if client.ValidRedirectURI(uri) {
		return uri, nil
	}

	return "", ErrInvalidRedirectURI
}

// DefaultGrantStrategy grants only empty scopes.
func DefaultGrantStrategy(_ Client, _ ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error) {
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
func DefaultApproveStrategy(Client, ResourceOwner, GenericToken, oauth2.Scope) (oauth2.Scope, error) {
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
		Secret:  []byte(secret),
		Token:   &Token{},
		Clients: []Client{&Application{}},
		Grants: func(Client) (Grants, error) {
			return Grants{}, nil
		},
		RedirectURIValidator: DefaultRedirectURIValidator,
		ResourceOwners: func(_ Client) ([]ResourceOwner, error) {
			return []ResourceOwner{&User{}}, nil
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

	// set user data
	var userData heat.Data
	if p.TokenData != nil {
		userData = p.TokenData(client, resourceOwner, token)
	}

	// issue key
	str, err := heat.Issue(p.Secret, "flame", "token", heat.RawKey{
		ID:     token.ID().Hex(),
		Expiry: data.ExpiresAt,
		Data:   userData,
	})
	if err != nil {
		return "", nil
	}

	return str, nil
}

// ParseJWT will parse the presented token and return its claims, if it is
// expired and eventual errors.
func (p *Policy) ParseJWT(str string) (*heat.RawKey, error) {
	// parse token and check expired errors
	key, err := heat.Verify(p.Secret, "flame", "token", str)
	if err != nil {
		return nil, err
	}

	// parse id
	_, err = coal.FromHex(key.ID)
	if err != nil {
		return nil, errors.New("invalid id")
	}

	return key, nil
}
