package flame

import (
	"context"
	"net/http"
	"time"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// ErrInvalidFilter should be returned by the ResourceOwnerFilter to indicate
// that the request includes invalid filter parameters.
var ErrInvalidFilter = xo.BF("invalid filter")

// ErrInvalidRedirectURI should be returned by the RedirectURIValidator to
// indicate that the redirect URI is invalid.
var ErrInvalidRedirectURI = xo.BF("invalid redirect uri")

// ErrGrantRejected should be returned by the GrantStrategy to indicate a rejection
// of the grant based on the provided conditions.
var ErrGrantRejected = xo.BF("grant rejected")

// ErrApprovalRejected should be returned by the ApproveStrategy to indicate a
// rejection of the approval based on the provided conditions.
var ErrApprovalRejected = xo.BF("approval rejected")

// ErrInvalidScope should be returned by the GrantStrategy to indicate that the
// requested scope exceeds the grantable scope.
var ErrInvalidScope = xo.BF("invalid scope")

// Key is they key used to issue and verify tokens and codes.
type Key struct {
	heat.Base `json:"-" heat:"fire/flame.key,1h"`

	// The extra data included in the key.
	Extra stick.Map `json:"extra,omitempty"`

	stick.NoValidation `json:"-"`
}

// Grants defines the selected grants.
type Grants struct {
	Password          bool
	ClientCredentials bool
	Implicit          bool
	AuthorizationCode bool
	RefreshToken      bool
}

// A Context provides useful contextual information.
type Context struct {
	// The context that is cancelled when the underlying connection transport
	// has been closed.
	//
	// Values: opentracing.Span, *xo.Tracer
	context.Context

	// The underlying HTTP request.
	//
	// Usage: Read Only
	Request *http.Request

	// The current tracer.
	//
	// Usage: Read Only
	Tracer *xo.Tracer

	writer http.ResponseWriter
	grants Grants
}

// Policy configures the provided authentication and authorization schemes used
// by the authenticator.
type Policy struct {
	// The notary used to issue and verify tokens and codes.
	Notary *heat.Notary

	// The token model.
	Token GenericToken

	// The client models.
	Clients []Client

	// Grants should return the permitted grants for the provided client.
	Grants func(ctx *Context, c Client) (Grants, error)

	// ClientFilter may return a filter that should be applied when looking
	// up a client. This callback can be used to select clients based on other
	// request parameters. It can return ErrInvalidFilter to cancel the
	// authentication request.
	ClientFilter func(ctx *Context, c Client) (bson.M, error)

	// RedirectURIValidator should validate a redirect URI and return the valid
	// or corrected redirect URI. It can return ErrInvalidRedirectURI to
	// cancel the authorization request. The validator is during the
	// authorization and the token request. If the result differs, no token will
	// be issue and the request aborted.
	RedirectURIValidator func(ctx *Context, c Client, redirectURI string) (string, error)

	// ResourceOwners should return a list of resource owner models that are
	// tried in order to resolve grant requests.
	ResourceOwners func(ctx *Context, c Client) ([]ResourceOwner, error)

	// ResourceOwnerFilter may return a filter that should be applied when
	// looking up a resource owner. This callback can be used to select resource
	// owners based on other request parameters. It can return ErrInvalidFilter
	// to cancel the authentication request.
	ResourceOwnerFilter func(ctx *Context, c Client, ro ResourceOwner) (bson.M, error)

	// GrantStrategy is invoked by the authenticator with the requested scope,
	// the client and the resource owner before issuing an access token. The
	// callback should return the scope that should be granted. It can return
	// ErrGrantRejected or ErrInvalidScope to cancel the grant request.
	//
	// Note: ResourceOwner is not set for a client credentials grant.
	GrantStrategy func(ctx *Context, c Client, ro ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error)

	// The URL to the page that obtains the approval of the user in implicit and
	// authorization code grants.
	ApprovalURL func(ctx *Context, c Client) (string, error)

	// ApproveStrategy is invoked by the authenticator to verify the
	// authorization approval by an authenticated resource owner in the implicit
	// grant and authorization code grant flows. The callback should return the
	// scope that should be granted. It may return ErrApprovalRejected or
	// ErrInvalidScope to cancel the approval request.
	//
	// Note: GenericToken represents the token that authorizes the resource
	// owner to give the approval.
	ApproveStrategy func(ctx *Context, c Client, ro ResourceOwner, token GenericToken, scope oauth2.Scope) (oauth2.Scope, error)

	// TokenData may return a map of data that should be included in the
	// generated JWT tokens as the "dat" field as well as in the token
	// introspection's response "extra" field.
	TokenData func(c Client, ro ResourceOwner, token GenericToken) map[string]interface{}

	// The token and code lifespans.
	AccessTokenLifespan       time.Duration
	RefreshTokenLifespan      time.Duration
	AuthorizationCodeLifespan time.Duration

	backTrackIssuedFromExpiry bool // TODO: Keep?
}

// StaticGrants always selects the specified grants.
func StaticGrants(password, clientCredentials, implicit, authorizationCode, refreshToken bool) func(*Context, Client) (Grants, error) {
	return func(*Context, Client) (Grants, error) {
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
func DefaultRedirectURIValidator(_ *Context, client Client, uri string) (string, error) {
	// check model
	if client.ValidRedirectURI(uri) {
		return uri, nil
	}

	return "", ErrInvalidRedirectURI.Wrap()
}

// DefaultGrantStrategy grants only empty scopes.
func DefaultGrantStrategy(_ *Context, _ Client, _ ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error) {
	// check scope
	if !scope.Empty() {
		return nil, ErrInvalidScope.Wrap()
	}

	return scope, nil
}

// StaticApprovalURL returns a static approval URL.
func StaticApprovalURL(url string) func(*Context, Client) (string, error) {
	return func(*Context, Client) (string, error) {
		return url, nil
	}
}

// DefaultApproveStrategy rejects all approvals.
func DefaultApproveStrategy(*Context, Client, ResourceOwner, GenericToken, oauth2.Scope) (oauth2.Scope, error) {
	return nil, ErrApprovalRejected.Wrap()
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
func DefaultPolicy(notary *heat.Notary) *Policy {
	return &Policy{
		Notary:  notary,
		Token:   &Token{},
		Clients: []Client{&Application{}},
		Grants: func(*Context, Client) (Grants, error) {
			return Grants{}, nil
		},
		RedirectURIValidator: DefaultRedirectURIValidator,
		ResourceOwners: func(_ *Context, _ Client) ([]ResourceOwner, error) {
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

// Issue will issue a JWT token based on the provided information.
func (p *Policy) Issue(token GenericToken, client Client, resourceOwner ResourceOwner) (string, error) {
	// get data
	data := token.GetTokenData()

	// get issued
	issued := time.Now()
	if p.backTrackIssuedFromExpiry && data.ExpiresAt.Before(issued) {
		issued = data.ExpiresAt.Add(-time.Hour)
	}

	// get extra data
	var extra stick.Map
	if p.TokenData != nil {
		extra = p.TokenData(client, resourceOwner, token)
	}

	// prepare key
	key := Key{
		Base: heat.Base{
			ID:      token.ID(),
			Issued:  issued,
			Expires: data.ExpiresAt,
		},
		Extra: extra,
	}

	// issue key
	str, err := p.Notary.Issue(&key)
	if err != nil {
		return "", err
	}

	return str, nil
}

// Verify will verify the presented token and return the decoded raw key.
func (p *Policy) Verify(str string) (*Key, error) {
	// parse token and check expired errors
	var key Key
	err := p.Notary.Verify(&key, str)
	if err != nil {
		return nil, err
	}

	return &key, nil
}
