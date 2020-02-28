// Package flame implements an authenticator that provides OAuth2 compatible
// authentication with JWT tokens.
package flame

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/stack"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/256dpi/fire/cinder"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

type ctxKey string

const (
	// AccessTokenContextKey is the key used to save the access token in a
	// context.
	AccessTokenContextKey = ctxKey("access-token")

	// ClientContextKey is the key used to save the client in a context.
	ClientContextKey = ctxKey("client")

	// ResourceOwnerContextKey is the key used to save the resource owner in a
	// context.
	ResourceOwnerContextKey = ctxKey("resource-owner")
)

type environment struct {
	request *http.Request
	writer  http.ResponseWriter
	trace   *cinder.Trace
	grants  Grants
}

// Authenticator provides OAuth2 based authentication and authorization. The
// implementation supports the standard "Resource Owner Credentials Grant",
// "Client Credentials Grant", "Implicit Grant" and "Authorization Code Grant".
// Additionally, it supports the "Refresh Token Grant" and "Token Revocation"
// flows.
type Authenticator struct {
	store    *coal.Store
	policy   *Policy
	reporter func(error)
}

// NewAuthenticator constructs a new Authenticator from a store and policy.
func NewAuthenticator(store *coal.Store, policy *Policy, reporter func(error)) *Authenticator {
	return &Authenticator{
		store:    store,
		policy:   policy,
		reporter: reporter,
	}
}

// Endpoint returns a handler for the common token and authorize endpoint.
func (a *Authenticator) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// create trace
		trace := cinder.New(r.Context(), "flame/Authenticator.Endpoint")
		trace.Tag("prefix", prefix)
		defer trace.Finish()

		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write oauth2 errors
			if oauth2Error, ok := err.(*oauth2.Error); ok {
				_ = oauth2.WriteError(w, oauth2Error)
				return
			}

			// set critical error on last span
			trace.Tag("error", true)
			trace.Log("error", err.Error())
			trace.Log("stack", stack.Trace())

			// otherwise report critical errors
			if a.reporter != nil {
				a.reporter(err)
			}

			// ignore errors caused by writing critical errors
			_ = oauth2.WriteError(w, oauth2.ServerError(""))
		})

		// trim and split path
		s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
		if len(s) != 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// prepare env
		env := &environment{
			request: r,
			writer:  w,
			trace:   trace,
		}

		// call endpoints
		switch s[0] {
		case "authorize":
			a.authorizationEndpoint(env)
		case "token":
			a.tokenEndpoint(env)
		case "revoke":
			a.revocationEndpoint(env)
		case "introspect":
			a.introspectionEndpoint(env)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// Authorizer returns a middleware that can be used to authorize a request by
// requiring an access token with the provided scope to be granted.
func (a *Authenticator) Authorizer(scope string, force, loadClient, loadResourceOwner bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// create trace
			trace := cinder.New(r.Context(), "flame/Authenticator.Authorizer")
			trace.Tag("scope", scope)
			trace.Tag("force", force)
			trace.Tag("loadClient", loadClient)
			trace.Tag("loadResourceOwner", loadResourceOwner)
			defer trace.Finish()

			// add span to context
			r = r.WithContext(trace.Wrap(r.Context()))

			// immediately pass on request if force is not set and there is
			// no authentication information provided
			if !force && r.Header.Get("Authorization") == "" {
				// call next handler
				next.ServeHTTP(w, r)

				return
			}

			// continue any previous aborts
			defer stack.Resume(func(err error) {
				// directly write bearer errors
				if bearerError, ok := err.(*oauth2.Error); ok {
					_ = oauth2.WriteBearerError(w, bearerError)
					return
				}

				// set critical error on last span
				trace.Tag("error", true)
				trace.Log("error", err.Error())
				trace.Log("stack", stack.Trace())

				// otherwise report critical errors
				if a.reporter != nil {
					a.reporter(err)
				}

				// write generic server error
				_ = oauth2.WriteBearerError(w, oauth2.ServerError(""))
			})

			// parse scope
			requiredScope := oauth2.ParseScope(scope)

			// parse bearer token
			tk, err := oauth2.ParseBearerToken(r)
			stack.AbortIf(err)

			// parse token
			key, err := a.policy.Verify(tk)
			if err == heat.ErrExpiredToken {
				stack.Abort(oauth2.InvalidToken("expired bearer token"))
			} else if err != nil {
				stack.Abort(oauth2.InvalidToken("malformed bearer token"))
			}

			// prepare env
			env := &environment{
				request: r,
				writer:  w,
				trace:   trace,
			}

			// get token
			accessToken := a.getToken(env, key.Base.ID)
			if accessToken == nil {
				stack.Abort(oauth2.InvalidToken("unknown bearer token"))
			}

			// get token data
			data := accessToken.GetTokenData()

			// validate token type
			if data.Type != AccessToken {
				stack.Abort(oauth2.InvalidToken("invalid bearer token type"))
			}

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				stack.Abort(oauth2.InvalidToken("expired access token"))
			}

			// validate scope
			if !data.Scope.Includes(requiredScope) {
				stack.Abort(oauth2.InsufficientScope(requiredScope.String()))
			}

			// create new context with access token
			ctx := context.WithValue(r.Context(), AccessTokenContextKey, accessToken)

			// call next handler if client should not be loaded
			if !loadClient {
				// call next handler
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}

			// get client
			client := a.getFirstClient(env, data.ClientID)
			if client == nil {
				stack.Abort(errors.New("missing client"))
			}

			// create new context with client
			ctx = context.WithValue(ctx, ClientContextKey, client)

			// call next handler if resource owner does not exist or should not
			// be loaded
			if data.ResourceOwnerID == nil || !loadResourceOwner {
				// call next handler
				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}

			// get resource owner
			resourceOwner := a.getFirstResourceOwner(env, client, *data.ResourceOwnerID)
			if resourceOwner == nil {
				stack.Abort(oauth2.InvalidToken("missing resource owner"))
			}

			// create new context with resource owner
			ctx = context.WithValue(ctx, ResourceOwnerContextKey, resourceOwner)

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Authenticator) authorizationEndpoint(env *environment) {
	// trace
	env.trace.Push("flame/Authenticator.authorizationEndpoint")
	defer env.trace.Pop()

	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(env.request)
	stack.AbortIf(err)

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		stack.Abort(oauth2.InvalidRequest("unknown response type"))
	}

	// get client
	client := a.findFirstClient(env, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate redirect URI
	req.RedirectURI, err = a.policy.RedirectURIValidator(client, req.RedirectURI)
	if err == ErrInvalidRedirectURI {
		stack.Abort(oauth2.InvalidRequest("invalid redirect uri"))
	} else if err != nil {
		stack.Abort(err)
	}

	// get grants
	env.grants, err = a.policy.Grants(client)
	stack.AbortIf(err)

	/* client is valid */

	// validate response type
	if req.ResponseType == oauth2.TokenResponseType && !env.grants.Implicit {
		stack.Abort(oauth2.UnsupportedResponseType(""))
	} else if req.ResponseType == oauth2.CodeResponseType && !env.grants.AuthorizationCode {
		stack.Abort(oauth2.UnsupportedResponseType(""))
	}

	// prepare abort method
	abort := func(err *oauth2.Error) {
		stack.Abort(err.SetRedirect(req.RedirectURI, req.State, req.ResponseType == oauth2.TokenResponseType))
	}

	// check request method
	if env.request.Method == "GET" {
		// get approval url
		url, err := a.policy.ApprovalURL(client)
		if err != nil {
			stack.Abort(err)
		} else if url == "" {
			abort(oauth2.InvalidRequest("unsupported request method"))
		}

		// prepare params
		params := map[string]string{}
		for name, values := range env.request.URL.Query() {
			params[name] = values[0]
		}

		// perform redirect
		stack.AbortIf(oauth2.WriteRedirect(env.writer, url, params, false))

		return
	}

	// get access token
	token := env.request.Form.Get("access_token")
	if token == "" {
		abort(oauth2.AccessDenied("missing access token"))
	}

	// parse token
	key, err := a.policy.Verify(token)
	if err == heat.ErrExpiredToken {
		abort(oauth2.AccessDenied("expired access token"))
	} else if err != nil {
		abort(oauth2.AccessDenied("invalid access token"))
	}

	// get token
	accessToken := a.getToken(env, key.Base.ID)
	if accessToken == nil {
		abort(oauth2.AccessDenied("unknown access token"))
	}

	// get token data
	data := accessToken.GetTokenData()

	// validate token type
	if data.Type != AccessToken {
		abort(oauth2.AccessDenied("invalid access token type"))
	}

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		abort(oauth2.AccessDenied("expired access token"))
	}

	// check resource owner
	if data.ResourceOwnerID == nil {
		abort(oauth2.AccessDenied("missing resource owner"))
	}

	// get resource owner
	resourceOwner := a.getFirstResourceOwner(env, client, *data.ResourceOwnerID)
	if resourceOwner == nil {
		abort(oauth2.AccessDenied("unknown resource owner"))
	}

	// validate & grant scope
	scope, err := a.policy.ApproveStrategy(client, resourceOwner, accessToken, req.Scope)
	if err == ErrApprovalRejected {
		abort(oauth2.AccessDenied("approval rejected"))
	} else if err == ErrInvalidScope {
		abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		// issue access token
		res := a.issueTokens(env, false, scope, req.RedirectURI, client, resourceOwner)
		res.SetRedirect(req.RedirectURI, req.State)

		// write response
		stack.AbortIf(oauth2.WriteTokenResponse(env.writer, res))
	case oauth2.CodeResponseType:
		// issue authorization code
		res := a.issueCode(env, scope, req.RedirectURI, client, resourceOwner)
		res.State = req.State

		// write response
		stack.AbortIf(oauth2.WriteCodeResponse(env.writer, res))
	}
}

func (a *Authenticator) tokenEndpoint(env *environment) {
	// trace
	env.trace.Push("flame/Authenticator.tokenEndpoint")
	defer env.trace.Pop()

	// parse token request
	req, err := oauth2.ParseTokenRequest(env.request)
	stack.AbortIf(err)

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		stack.Abort(oauth2.InvalidRequest("unknown grant type"))
	}

	// get client
	client := a.findFirstClient(env, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// get grants
	env.grants, err = a.policy.Grants(client)
	stack.AbortIf(err)

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		// check availability
		if !env.grants.Password {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle resource owner password credentials grant
		a.handleResourceOwnerPasswordCredentialsGrant(env, req, client)
	case oauth2.ClientCredentialsGrantType:
		// check availability
		if !env.grants.ClientCredentials {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle client credentials grant
		a.handleClientCredentialsGrant(env, req, client)
	case oauth2.RefreshTokenGrantType:
		// check availability
		if !env.grants.RefreshToken {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle refresh token grant
		a.handleRefreshTokenGrant(env, req, client)
	case oauth2.AuthorizationCodeGrantType:
		// check availability
		if !env.grants.AuthorizationCode {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle authorization code grant
		a.handleAuthorizationCodeGrant(env, req, client)
	}
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// trace
	env.trace.Push("flame/Authenticator.handleResourceOwnerPasswordCredentialsGrant")
	defer env.trace.Pop()

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// get resource owner
	resourceOwner := a.findFirstResourceOwner(env, client, req.Username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied("")) // never expose reason!
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		stack.Abort(oauth2.AccessDenied("")) // never expose reason!
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(client, resourceOwner, req.Scope)
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied("")) // never expose reason!
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(env, true, scope, "", client, resourceOwner)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(env.writer, res))
}

func (a *Authenticator) handleClientCredentialsGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// trace
	env.trace.Push("flame/Authenticator.handleClientCredentialsGrant")
	defer env.trace.Pop()

	// check confidentiality
	if !client.IsConfidential() {
		stack.Abort(oauth2.InvalidClient("non confidential client"))
	}

	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(client, nil, req.Scope)
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied("grant rejected"))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(env, true, scope, "", client, nil)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(env.writer, res))
}

func (a *Authenticator) handleRefreshTokenGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// trace
	env.trace.Push("flame/Authenticator.handleRefreshTokenGrant")
	defer env.trace.Pop()

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	key, err := a.policy.Verify(req.RefreshToken)
	if err == heat.ErrExpiredToken {
		stack.Abort(oauth2.InvalidGrant("expired refresh token"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed refresh token"))
	}

	// get stored refresh token by signature
	rt := a.getToken(env, key.Base.ID)
	if rt == nil {
		stack.Abort(oauth2.InvalidGrant("unknown refresh token"))
	}

	// get token data
	data := rt.GetTokenData()

	// validate type
	if data.Type != RefreshToken {
		stack.Abort(oauth2.InvalidGrant("invalid refresh token type"))
	}

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		stack.Abort(oauth2.InvalidGrant("expired refresh token"))
	}

	// validate ownership
	if data.ClientID != client.ID() {
		stack.Abort(oauth2.InvalidGrant("invalid refresh token ownership"))
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !data.Scope.Includes(req.Scope) {
		stack.Abort(oauth2.InvalidScope("scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = a.getFirstResourceOwner(env, client, *data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(env, true, req.Scope, data.RedirectURI, client, ro)

	// delete refresh token
	a.deleteToken(env, rt.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(env.writer, res))
}

func (a *Authenticator) handleAuthorizationCodeGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// trace
	env.trace.Push("flame/Authenticator.handleAuthorizationCodeGrant")
	defer env.trace.Pop()

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse authorization code
	key, err := a.policy.Verify(req.Code)
	if err == heat.ErrExpiredToken {
		stack.Abort(oauth2.InvalidGrant("expired authorization code"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed authorization code"))
	}

	// TODO: We should revoke all descending tokens if a code is reused.

	// get stored authorization code by signature
	code := a.getToken(env, key.Base.ID)

	if code == nil {
		stack.Abort(oauth2.InvalidGrant("unknown authorization code"))
	}

	// get token data
	data := code.GetTokenData()

	// validate type
	if data.Type != AuthorizationCode {
		stack.Abort(oauth2.InvalidGrant("invalid authorization code type"))
	}

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		stack.Abort(oauth2.InvalidGrant("expired authorization code"))
	}

	// validate ownership
	if data.ClientID != client.ID() {
		stack.Abort(oauth2.InvalidGrant("invalid authorization code ownership"))
	}

	// validate redirect URI
	req.RedirectURI, err = a.policy.RedirectURIValidator(client, req.RedirectURI)
	if err == ErrInvalidRedirectURI {
		stack.Abort(oauth2.InvalidRequest("invalid redirect uri"))
	} else if err != nil {
		stack.Abort(err)
	}

	// compare redirect URIs
	if data.RedirectURI != req.RedirectURI {
		stack.Abort(oauth2.InvalidGrant("redirect uri mismatch"))
	}

	// inherit scope from stored authorization code
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !data.Scope.Includes(req.Scope) {
		stack.Abort(oauth2.InvalidScope("scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = a.getFirstResourceOwner(env, client, *data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(env, true, req.Scope, data.RedirectURI, client, ro)

	// delete authorization code
	a.deleteToken(env, code.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(env.writer, res))
}

func (a *Authenticator) revocationEndpoint(env *environment) {
	// trace
	env.trace.Push("flame/Authenticator.revocationEndpoint")
	defer env.trace.Pop()

	// parse authorization request
	req, err := oauth2.ParseRevocationRequest(env.request)
	stack.AbortIf(err)

	// check token type hint
	if req.TokenTypeHint != "" && !oauth2.KnownTokenType(req.TokenTypeHint) {
		stack.Abort(oauth2.UnsupportedTokenType(""))
	}

	// get client
	client := a.findFirstClient(env, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	key, err := a.policy.Verify(req.Token)
	if err == heat.ErrExpiredToken {
		env.writer.WriteHeader(http.StatusOK)
		return
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// get token
	token := a.getToken(env, key.Base.ID)
	if token != nil {
		// get data
		data := token.GetTokenData()

		// check ownership
		if data.ClientID != client.ID() {
			stack.Abort(oauth2.InvalidClient("wrong client"))
			return
		}

		// delete token
		a.deleteToken(env, key.Base.ID)
	}

	// write header
	env.writer.WriteHeader(http.StatusOK)
}

func (a *Authenticator) introspectionEndpoint(env *environment) {
	// trace
	env.trace.Push("flame/Authenticator.introspectionEndpoint")
	defer env.trace.Pop()

	// parse introspection request
	req, err := oauth2.ParseIntrospectionRequest(env.request)
	stack.AbortIf(err)

	// check token type hint
	if req.TokenTypeHint != "" && !oauth2.KnownTokenType(req.TokenTypeHint) {
		stack.Abort(oauth2.UnsupportedTokenType(""))
	}

	// get client
	client := a.findFirstClient(env, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	key, err := a.policy.Verify(req.Token)
	if err == heat.ErrExpiredToken {
		stack.AbortIf(oauth2.WriteIntrospectionResponse(env.writer, &oauth2.IntrospectionResponse{}))
		return
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// prepare response
	res := &oauth2.IntrospectionResponse{}

	// get token
	token := a.getToken(env, key.Base.ID)
	if token != nil {
		// get data
		data := token.GetTokenData()

		// check ownership
		if data.ClientID != client.ID() {
			stack.Abort(oauth2.InvalidClient("wrong client"))
			return
		}

		// get resource owner
		var resourceOwner ResourceOwner
		if data.ResourceOwnerID != nil {
			resourceOwner = a.getFirstResourceOwner(env, client, *data.ResourceOwnerID)
		}

		// get validity
		expired := data.ExpiresAt.Before(time.Now())

		// set response if valid and can be introspected
		if !expired && (data.Type == AccessToken || data.Type == RefreshToken) {
			res.Active = true
			res.Scope = data.Scope.String()
			res.ClientID = data.ClientID.Hex()
			if data.ResourceOwnerID != nil {
				res.Username = data.ResourceOwnerID.Hex()
			}
			res.TokenType = oauth2.AccessToken
			if data.Type == RefreshToken {
				res.TokenType = oauth2.RefreshToken
			}
			res.ExpiresAt = data.ExpiresAt.Unix()
			res.IssuedAt = token.ID().Timestamp().Unix()
			res.Identifier = token.ID().Hex()
			res.Extra = a.policy.TokenData(client, resourceOwner, token)
		}
	}

	// write response
	stack.AbortIf(oauth2.WriteIntrospectionResponse(env.writer, res))
}

func (a *Authenticator) issueTokens(env *environment, refreshable bool, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
	// trace
	env.trace.Push("flame/Authenticator.issueTokens")
	defer env.trace.Pop()

	// prepare expiration
	atExpiry := time.Now().Add(a.policy.AccessTokenLifespan)
	rtExpiry := time.Now().Add(a.policy.RefreshTokenLifespan)

	// save access token
	at := a.saveToken(env, AccessToken, scope, atExpiry, redirectURI, client, resourceOwner)

	// generate new access token
	atSignature, err := a.policy.Issue(at, client, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := oauth2.NewBearerTokenResponse(atSignature, int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	// issue a refresh token if requested
	if refreshable && env.grants.RefreshToken {
		// save refresh token
		rt := a.saveToken(env, RefreshToken, scope, rtExpiry, redirectURI, client, resourceOwner)

		// generate new refresh token
		rtSignature, err := a.policy.Issue(rt, client, resourceOwner)
		stack.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	return res
}

func (a *Authenticator) issueCode(env *environment, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.CodeResponse {
	// trace
	env.trace.Push("flame/Authenticator.issueCode")
	defer env.trace.Pop()

	// prepare expiration
	expiry := time.Now().Add(a.policy.AuthorizationCodeLifespan)

	// save authorization code
	code := a.saveToken(env, AuthorizationCode, scope, expiry, redirectURI, client, resourceOwner)

	// generate new access token
	signature, err := a.policy.Issue(code, client, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := oauth2.NewCodeResponse(signature, redirectURI, "")

	return res
}

func (a *Authenticator) findFirstClient(env *environment, id string) Client {
	// trace
	env.trace.Push("flame/Authenticator.findFirstClient")
	defer env.trace.Pop()

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.findClient(env, model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) findClient(env *environment, model Client, id string) Client {
	// trace
	env.trace.Push("flame/Authenticator.findClient")
	defer env.trace.Pop()

	// prepare client
	client := coal.GetMeta(model).Make().(Client)

	// use tagged field if present
	var filters []bson.M
	idField := coal.L(model, "flame-client-id", false)
	if idField != "" {
		filters = []bson.M{
			{coal.F(model, idField): id},
		}
	} else if coal.IsHex(id) {
		filters = []bson.M{
			{"_id": coal.MustFromHex(id)},
		}
	} else {
		stack.Abort(fmt.Errorf("unable to determine client id field"))
	}

	// add additional filter if provided
	if a.policy.ClientFilter != nil {
		// run filter function
		filter, err := a.policy.ClientFilter(model, env.request)
		if err == ErrInvalidFilter {
			stack.Abort(oauth2.InvalidRequest("invalid filter"))
		} else if err != nil {
			stack.Abort(err)
		}

		// add filter if present
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	// prepare query
	query := bson.M{
		"$and": filters,
	}

	// fetch client
	err := a.store.TC(model, env.trace).FindOne(nil, query).Decode(client)
	if coal.IsMissing(err) {
		return nil
	}
	stack.AbortIf(err)

	return client
}

func (a *Authenticator) getFirstClient(env *environment, id coal.ID) Client {
	// trace
	env.trace.Push("flame/Authenticator.getFirstClient")
	defer env.trace.Pop()

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.getClient(env, model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) getClient(env *environment, model Client, id coal.ID) Client {
	// trace
	env.trace.Push("flame/Authenticator.getClient")
	defer env.trace.Pop()

	// prepare client
	client := coal.GetMeta(model).Make().(Client)

	// fetch client
	err := a.store.TC(model, env.trace).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(client)
	if coal.IsMissing(err) {
		return nil
	}
	stack.AbortIf(err)

	return client
}

func (a *Authenticator) findFirstResourceOwner(env *environment, client Client, id string) ResourceOwner {
	// trace
	env.trace.Push("flame/Authenticator.findFirstResourceOwner")
	defer env.trace.Pop()

	// get resource owners
	resourceOwners, err := a.policy.ResourceOwners(client)
	stack.AbortIf(err)

	// check all available models in order
	for _, model := range resourceOwners {
		ro := a.findResourceOwner(env, client, model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) findResourceOwner(env *environment, client Client, model ResourceOwner, id string) ResourceOwner {
	// trace
	env.trace.Push("flame/Authenticator.findResourceOwner")
	defer env.trace.Pop()

	// prepare resource owner
	resourceOwner := coal.GetMeta(model).Make().(ResourceOwner)

	// use tagged field if present
	var filters []bson.M
	idField := coal.L(model, "flame-resource-owner-id", false)
	if idField != "" {
		filters = []bson.M{
			{coal.F(model, idField): id},
		}
	} else if coal.IsHex(id) {
		filters = []bson.M{
			{"_id": coal.MustFromHex(id)},
		}
	} else {
		stack.Abort(fmt.Errorf("unable to determine resource owner id field"))
	}

	// add additional filter if provided
	if a.policy.ResourceOwnerFilter != nil {
		// run filter function
		filter, err := a.policy.ResourceOwnerFilter(client, model, env.request)
		if err == ErrInvalidFilter {
			stack.Abort(oauth2.InvalidRequest("invalid filter"))
		} else if err != nil {
			stack.Abort(err)
		}

		// add filter if present
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	// prepare query
	query := bson.M{
		"$and": filters,
	}

	// fetch resource owner
	err := a.store.TC(model, env.trace).FindOne(nil, query).Decode(resourceOwner)
	if coal.IsMissing(err) {
		return nil
	}
	stack.AbortIf(err)

	return resourceOwner
}

func (a *Authenticator) getFirstResourceOwner(env *environment, client Client, id coal.ID) ResourceOwner {
	// trace
	env.trace.Push("flame/Authenticator.getFirstResourceOwner")
	defer env.trace.Pop()

	// get resource owners
	resourceOwners, err := a.policy.ResourceOwners(client)
	stack.AbortIf(err)

	// check all available models in order
	for _, model := range resourceOwners {
		ro := a.getResourceOwner(env, model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) getResourceOwner(env *environment, model ResourceOwner, id coal.ID) ResourceOwner {
	// trace
	env.trace.Push("flame/Authenticator.getResourceOwner")
	defer env.trace.Pop()

	// prepare object
	resourceOwner := coal.GetMeta(model).Make().(ResourceOwner)

	// fetch resource owner
	err := a.store.TC(model, env.trace).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(resourceOwner)
	if coal.IsMissing(err) {
		return nil
	}
	stack.AbortIf(err)

	return resourceOwner
}

func (a *Authenticator) getToken(env *environment, id coal.ID) GenericToken {
	// trace
	env.trace.Push("flame/Authenticator.getToken")
	defer env.trace.Pop()

	// prepare object
	token := coal.GetMeta(a.policy.Token).Make().(GenericToken)

	// fetch token
	err := a.store.TC(token, env.trace).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(token)
	if coal.IsMissing(err) {
		return nil
	}
	stack.AbortIf(err)

	return token
}

func (a *Authenticator) saveToken(env *environment, typ TokenType, scope []string, expiresAt time.Time, redirectURI string, client Client, resourceOwner ResourceOwner) GenericToken {
	// trace
	env.trace.Push("flame/Authenticator.saveToken")
	defer env.trace.Pop()

	// create token with id
	token := coal.GetMeta(a.policy.Token).Make().(GenericToken)
	token.GetBase().DocID = coal.New()

	// get resource owner id
	var roID *coal.ID
	if resourceOwner != nil {
		roID = coal.P(resourceOwner.ID())
	}

	// set token data
	token.SetTokenData(TokenData{
		Type:            typ,
		Scope:           scope,
		ExpiresAt:       expiresAt,
		RedirectURI:     redirectURI,
		Client:          client,
		ResourceOwner:   resourceOwner,
		ClientID:        client.ID(),
		ResourceOwnerID: roID,
	})

	// save token
	_, err := a.store.TC(token, env.trace).InsertOne(nil, token)
	stack.AbortIf(err)

	return token
}

func (a *Authenticator) deleteToken(env *environment, id coal.ID) {
	// trace
	env.trace.Push("flame/Authenticator.deleteToken")
	defer env.trace.Pop()

	// delete token
	_, err := a.store.TC(a.policy.Token, env.trace).DeleteOne(nil, bson.M{
		"_id": id,
	})
	if coal.IsMissing(err) {
		err = nil
	}
	stack.AbortIf(err)
}
