// Package oauth2 implements an authenticator component that provides OAuth2
// compatible authentication.
package auth

import (
	"fmt"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/model"
	"github.com/gonfire/oauth2/hmacsha"
	"github.com/labstack/echo"
)

var _ fire.RoutableComponent = (*Authenticator)(nil)

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant.
type Authenticator struct {
	prefix string

	Policy  *Policy
	Storage *Storage
}

// New constructs a new Authenticator.
func New(store *model.Store, policy *Policy, prefix string) *Authenticator {
	// check secret
	if len(policy.Secret) < 16 {
		panic("Secret must be longer than 16 characters")
	}

	// initialize models
	model.Init(policy.AccessToken)
	model.Init(policy.RefreshToken)
	model.Init(policy.Client)
	model.Init(policy.ResourceOwner)

	// create storage
	storage := NewStorage(policy, store)

	return &Authenticator{
		prefix:  prefix,
		Policy:  policy,
		Storage: storage,
	}
}

// Register implements the fire.RoutableComponent interface.
func (a *Authenticator) Register(router *echo.Echo) {
	//router.POST(a.prefix+"/token", a.TokenEndpoint)
	//router.POST(a.prefix+"/authorize", a.AuthorizationEndpoint)
}

// NewKeyAndSignature returns a new key with a matching signature that can be
// used to issue custom access tokens.
func (a *Authenticator) NewKeyAndSignature() (string, string, error) {
	token, err := hmacsha.Generate(a.Policy.Secret, 32)
	if err != nil {
		return "", "", err
	}

	return token.String(), token.SignatureString(), nil
}

/*
// Authorize can be used to authorize a request by requiring an access token with
// the provided scopes to be granted.
func (a *Authenticator) Authorize(ctx echo.Context, scopes []string) error {
	// create new session
	session := &oauth2.HMACSession{}

	// get underlying http request
	r := ctx.Request().(*standard.Request).Request

	// get token
	token := fosite.AccessTokenFromRequest(r)

	// validate request
	_, err := a.provider.ValidateToken(contextForContext(ctx), token, fosite.AccessToken, session, scopes...)
	if err != nil {
		return err
	}

	return nil
}
*/

// Authorizer returns a callback that can be used to protect resources by
// requiring an access token with the provided scopes to be granted.
/*func (a *Authenticator) Authorizer(scopes ...string) jsonapi.Callback {
	if len(scopes) < 1 {
		panic("Authorizer must be called with at least one scope")
	}

	return func(ctx *jsonapi.Context) error {
		return a.Authorize(ctx.Echo, scopes)
	}
}*/

// Describe implements the fire.Component interface.
func (a *Authenticator) Describe() fire.ComponentInfo {
	return fire.ComponentInfo{
		Name: "Authenticator",
		Settings: fire.Map{
			"Prefix":                         a.prefix,
			"Allow Password Grant":           fmt.Sprintf("%v", a.Policy.PasswordGrant),
			"Allow Client Credentials Grant": fmt.Sprintf("%v", a.Policy.ClientCredentialsGrant),
			"Allow Implicit Grant":           fmt.Sprintf("%v", a.Policy.ImplicitGrant),
			"Access Token Lifespan":          a.Policy.AccessTokenLifespan.String(),
			"Refresh Token Lifespan":         a.Policy.RefreshTokenLifespan.String(),
			"Access Token Model":             a.Policy.AccessToken.Meta().Name,
			"Refresh Token Model":            a.Policy.RefreshToken.Meta().Name,
			"Client Model":                   a.Policy.Client.Meta().Name,
			"Resource Owner Model":           a.Policy.ResourceOwner.Meta().Name,
		},
	}
}
