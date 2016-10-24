// Package oauth2 implements an authenticator component that provides OAuth2
// compatible authentication.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/jsonapi"
	"github.com/gonfire/fire/model"
	"github.com/gonfire/oauth2"
	"github.com/gonfire/oauth2/bearer"
	"github.com/gonfire/oauth2/hmacsha"
	"github.com/pressly/chi"
)

var _ fire.RoutableComponent = (*Authenticator)(nil)

const AccessTokenContextKey = "fire.oauth2.access_token"

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
func (a *Authenticator) Register(_ *fire.Application, router chi.Router) {
	router.HandleFunc(a.prefix+"/token", a.TokenEndpoint)
	router.HandleFunc(a.prefix+"/authorize", a.AuthorizationEndpoint)
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

// Authorize can be used to authorize a request by requiring an access token with
// the provided scopes to be granted. The method returns a middleware that can be
// called before any other routes.
func (a *Authenticator) Authorize(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// parse scope
			s := oauth2.ParseScope(scope)

			// parse bearer token
			tk, res := bearer.ParseToken(r)
			if res != nil {
				bearer.WriteError(w, res)
				return
			}

			// parse token
			token, err := hmacsha.Parse(a.Policy.Secret, tk)
			if err != nil {
				bearer.WriteError(w, bearer.InvalidToken("Malformed token"))
				return
			}

			// get token
			accessToken, err := a.Storage.GetAccessToken(token.SignatureString())
			if err != nil {
				bearer.WriteError(w, err)
				return
			} else if accessToken == nil {
				bearer.WriteError(w, bearer.InvalidToken("Unkown token"))
				return
			}

			// get additional data
			data := accessToken.GetTokenData()

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				bearer.WriteError(w, bearer.InvalidToken("Expired token"))
				return
			}

			// validate scope
			if !data.Scope.Includes(s) {
				bearer.WriteError(w, bearer.InsufficientScope(s.String()))
				return
			}

			// create new context with access token
			ctx := context.WithValue(r.Context(), AccessTokenContextKey, accessToken)

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authorizer returns a callback that can be used to protect resources by
// requiring an access token with the provided scopes to be granted.
//
// Note: Authorizer requires that the request has already been processed by
// Authorize.
func (a *Authenticator) Authorizer(scope string) jsonapi.Callback {
	return func(ctx *jsonapi.Context) error {
		// parse scope
		s := oauth2.ParseScope(scope)

		// get access token
		accessToken := ctx.HTTPRequest.Context().Value(AccessTokenContextKey).(Token)
		if accessToken == nil {
			return jsonapi.Fatal(errors.New("missing access token"))
		}

		// validate scope
		if !accessToken.GetTokenData().Scope.Includes(s) {
			return errors.New("unauthorized")
		}

		return nil
	}
}

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
