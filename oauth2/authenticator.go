// Package oauth2 implements an authenticator component that provides OAuth2
// compatible authentication.
package oauth2

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/jsonapi"
	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/compose"
	"github.com/ory-am/fosite/handler/oauth2"
	"golang.org/x/crypto/bcrypt"
)

// the default hash cost that is used by the token hasher
var hashCost = bcrypt.DefaultCost

const (
	// PasswordGrant specifies the OAuth Resource Owner Password Credentials Grant.
	PasswordGrant = "password"

	// ClientCredentialsGrant specifies the OAuth Client Credentials Grant.
	ClientCredentialsGrant = "client_credentials"

	// ImplicitGrant specifies the OAuth Implicit Grant.
	ImplicitGrant = "implicit"
)

var _ fire.RoutableComponent = (*Authenticator)(nil)

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant flows.
type Authenticator struct {
	store  *model.Store
	prefix string
	policy *Policy

	config   *compose.Config
	provider *fosite.Fosite
	strategy *oauth2.HMACSHAStrategy
	storage  *storage
}

// New creates and returns a new Authenticator.
func New(store *model.Store, policy *Policy, prefix string) *Authenticator {
	// check secret
	if len(policy.Secret) < 16 {
		panic("Secret must be longer than 16 characters")
	}

	// initialize models
	model.Init(policy.OwnerModel)
	model.Init(policy.ClientModel)
	model.Init(policy.AccessTokenModel)

	// create storage
	storage := &storage{}

	// provider config
	config := &compose.Config{
		AccessTokenLifespan: policy.TokenLifespan,
		HashCost:            hashCost,
	}

	// create a new token generation strategy
	strategy := compose.NewOAuth2HMACStrategy(config, policy.Secret)

	// create provider
	provider := compose.Compose(config, storage, strategy).(*fosite.Fosite)

	// add password grant handler
	if policy.PasswordGrant {
		grantHandler := compose.OAuth2ResourceOwnerPasswordCredentialsFactory(config, storage, strategy)
		provider.TokenEndpointHandlers.Append(grantHandler.(fosite.TokenEndpointHandler))
		provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))
	}

	// add client credentials grant handler
	if policy.ClientCredentialsGrant {
		grantHandler := compose.OAuth2ClientCredentialsGrantFactory(config, storage, strategy)
		provider.TokenEndpointHandlers.Append(grantHandler.(fosite.TokenEndpointHandler))
		provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))
	}

	// add implicit grant handler
	if policy.ImplicitGrant {
		grantHandler := compose.OAuth2AuthorizeImplicitFactory(config, storage, strategy)
		provider.AuthorizeEndpointHandlers.Append(grantHandler.(fosite.AuthorizeEndpointHandler))
		provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))
	}

	// create authenticator
	a := &Authenticator{
		store:    store,
		prefix:   prefix,
		policy:   policy,
		config:   config,
		provider: provider,
		strategy: strategy,
		storage:  storage,
	}

	// set authenticator
	storage.authenticator = a

	return a
}

// Register implements the fire.RoutableComponent interface.
func (a *Authenticator) Register(router *echo.Echo) {
	router.POST(a.prefix+"/token", a.tokenEndpoint)
	router.POST(a.prefix+"/authorize", a.authorizeEndpoint)
}

// NewKeyAndSignature returns a new key with a matching signature that can be
// used to issue custom access tokens.
func (a *Authenticator) NewKeyAndSignature() (string, string, error) {
	return a.strategy.GenerateAccessToken(nil, nil)
}

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

// Authorizer returns a callback that can be used to protect resources by
// requiring an access token with the provided scopes to be granted.
func (a *Authenticator) Authorizer(scopes ...string) jsonapi.Callback {
	if len(scopes) < 1 {
		panic("Authorizer must be called with at least one scope")
	}

	return func(ctx *jsonapi.Context) error {
		return a.Authorize(ctx.Echo, scopes)
	}
}

// EchoAuthorizer can be used to protect echo handlers by requiring an access
// token with the provided scopes to be granted.
func (a *Authenticator) EchoAuthorizer(scopes ...string) echo.MiddlewareFunc {
	if len(scopes) < 1 {
		panic("EchoAuthorizer must be called with at least one scope")
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			err := a.Authorize(ctx, scopes)
			if err != nil {
				return ctx.NoContent(http.StatusUnauthorized)
			}

			return next(ctx)
		}
	}
}

// Describe implements the fire.Component interface.
func (a *Authenticator) Describe() fire.ComponentInfo {
	return fire.ComponentInfo{
		Name: "OAuth2 Authenticator",
		Settings: fire.Map{
			"Prefix":                         a.prefix,
			"Allow Password Grant":           fmt.Sprintf("%v", a.policy.PasswordGrant),
			"Allow Client Credentials Grant": fmt.Sprintf("%v", a.policy.ClientCredentialsGrant),
			"Allow Implicit Grant":           fmt.Sprintf("%v", a.policy.ImplicitGrant),
			"Token Lifespan":                 a.policy.TokenLifespan.String(),
			"Access Token Model":             a.policy.AccessTokenModel.Meta().Name,
			"Client Model":                   a.policy.ClientModel.Meta().Name,
			"Owner Model":                    a.policy.OwnerModel.Meta().Name,
		},
	}
}

func (a *Authenticator) tokenEndpoint(ctx echo.Context) error {
	var err error

	// create new session
	session := &oauth2.HMACSession{}

	// get underlying http request
	r := ctx.Request().(*standard.Request).Request
	w := ctx.Response().(*standard.Response).ResponseWriter

	// obtain access request
	req, err := a.provider.NewAccessRequest(contextForContext(ctx), r, session)
	if err != nil {
		a.provider.WriteAccessError(w, req, err)

		if isFatalError(err) {
			return err
		}

		return nil
	}

	// extract grant type
	grantType := req.GetGrantTypes()[0]

	// retrieve and set client
	client := req.GetClient().(*abstractClient).model
	ctx.Set("client", client)

	// retrieve optional owner
	var owner OwnerModel
	if val, ok := ctx.Get("owner").(OwnerModel); ok {
		owner = val
	}

	// grant scopes
	a.invokeGrantStrategy(grantType, req, client, owner)

	// obtain access response
	res, err := a.provider.NewAccessResponse(contextForContext(ctx), r, req)
	if err != nil {
		a.provider.WriteAccessError(w, req, err)

		if isFatalError(err) {
			return err
		}

		return nil
	}

	// write response
	a.provider.WriteAccessResponse(w, req, res)
	return nil
}

func (a *Authenticator) authorizeEndpoint(ctx echo.Context) error {
	// get underlying http request
	r := ctx.Request().(*standard.Request).Request
	w := ctx.Response().(*standard.Response).ResponseWriter

	// obtain authorize request
	req, err := a.provider.NewAuthorizeRequest(contextForContext(ctx), r)
	if err != nil {
		a.provider.WriteAuthorizeError(w, req, err)

		if isFatalError(err) {
			return err
		}

		return nil
	}

	// get credentials
	username := ctx.FormValue("username")
	password := ctx.FormValue("password")

	// authenticate user
	err = a.storage.Authenticate(contextForContext(ctx), username, password)
	if err != nil {
		a.provider.WriteAuthorizeError(w, req, fosite.ErrAccessDenied)
		return nil
	}

	// retrieve and set models
	owner := ctx.Get("owner").(OwnerModel)
	client := req.GetClient().(*abstractClient).model
	ctx.Set("client", client)

	// check if client has all scopes
	for _, scope := range req.GetRequestedScopes() {
		if !a.provider.ScopeStrategy(req.GetClient().GetScopes(), scope) {
			a.provider.WriteAuthorizeError(w, req, fosite.ErrInvalidScope)
			return nil
		}
	}

	// grant scopes
	a.invokeGrantStrategy(ImplicitGrant, req, client, owner)

	// create new session
	session := &oauth2.HMACSession{}

	// obtain authorize response
	res, err := a.provider.NewAuthorizeResponse(contextForContext(ctx), r, req, session)
	if err != nil {
		a.provider.WriteAuthorizeError(w, req, err)

		if isFatalError(err) {
			return err
		}

		return nil
	}

	// write response
	a.provider.WriteAuthorizeResponse(w, req, res)
	return nil
}

func (a *Authenticator) invokeGrantStrategy(grantType string, req fosite.Requester, client ClientModel, owner OwnerModel) {
	grantedScopes := a.policy.GrantStrategy(&GrantRequest{
		GrantType:       grantType,
		RequestedScopes: req.GetRequestedScopes(),
		Client:          client,
		Owner:           owner,
	})

	for _, scope := range grantedScopes {
		req.GrantScope(scope)
	}
}

func contextForContext(ctx echo.Context) context.Context {
	return context.WithValue(ctx.StdContext(), "echo", ctx)
}

func isFatalError(err error) bool {
	return fosite.ErrorToRFC6749Error(err).StatusCode == http.StatusInternalServerError
}
