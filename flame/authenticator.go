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

	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/bearer"
	"github.com/256dpi/oauth2/introspection"
	"github.com/256dpi/oauth2/revocation"
	"github.com/256dpi/stack"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
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
	tracer  *fire.Tracer
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
	// initialize token
	coal.Init(policy.Token)

	// initialize clients
	for _, model := range policy.Clients {
		coal.Init(model)
	}

	return &Authenticator{
		store:    store,
		policy:   policy,
		reporter: reporter,
	}
}

// Endpoint returns a handler for the common token and authorize endpoint.
func (a *Authenticator) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// create tracer
		tracer := fire.NewTracerFromRequest(r, "flame/Authenticator.Endpoint")
		tracer.Tag("prefix", prefix)
		defer tracer.Finish(true)

		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write oauth2 errors
			if oauth2Error, ok := err.(*oauth2.Error); ok {
				_ = oauth2.WriteError(w, oauth2Error)
				return
			}

			// set critical error on last span
			tracer.Tag("error", true)
			tracer.Log("error", err.Error())
			tracer.Log("stack", stack.Trace())

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
			tracer:  tracer,
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
			// create tracer
			tracer := fire.NewTracerFromRequest(r, "flame/Authenticator.Authorizer")
			tracer.Tag("scope", scope)
			tracer.Tag("force", force)
			tracer.Tag("loadClient", loadClient)
			tracer.Tag("loadResourceOwner", loadResourceOwner)
			defer tracer.Finish(true)

			// add span to context
			r = r.WithContext(tracer.Context(r.Context()))

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
				if bearerError, ok := err.(*bearer.Error); ok {
					_ = bearer.WriteError(w, bearerError)
					return
				}

				// set critical error on last span
				tracer.Tag("error", true)
				tracer.Log("error", err.Error())
				tracer.Log("stack", stack.Trace())

				// otherwise report critical errors
				if a.reporter != nil {
					a.reporter(err)
				}

				// write generic server error
				_ = bearer.WriteError(w, bearer.ServerError())
			})

			// parse scope
			requiredScope := oauth2.ParseScope(scope)

			// parse bearer token
			tk, err := bearer.ParseToken(r)
			stack.AbortIf(err)

			// parse token
			claims, expired, err := a.policy.ParseJWT(tk)
			if expired {
				stack.Abort(bearer.InvalidToken("expired bearer token"))
			} else if err != nil {
				stack.Abort(bearer.InvalidToken("malformed bearer token"))
			}

			// prepare env
			env := &environment{
				request: r,
				writer:  w,
				tracer:  tracer,
			}

			// get id
			id, err := coal.FromHex(claims.Id)
			if err != nil {
				stack.Abort(bearer.InvalidToken("invalid bearer token id"))
			}

			// get token
			accessToken := a.getToken(env, id)
			if accessToken == nil {
				stack.Abort(bearer.InvalidToken("unknown bearer token"))
			}

			// get token data
			data := accessToken.GetTokenData()

			// validate token type
			if data.Type != AccessToken {
				stack.Abort(bearer.InvalidToken("invalid bearer token type"))
			}

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				stack.Abort(bearer.InvalidToken("expired access token"))
			}

			// validate scope
			if !data.Scope.Includes(requiredScope) {
				stack.Abort(bearer.InsufficientScope(requiredScope.String()))
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
				stack.Abort(bearer.InvalidToken("missing resource owner"))
			}

			// create new context with resource owner
			ctx = context.WithValue(ctx, ResourceOwnerContextKey, resourceOwner)

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Authenticator) authorizationEndpoint(env *environment) {
	// begin trace
	env.tracer.Push("flame/Authenticator.authorizationEndpoint")

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
	claims, expired, err := a.policy.ParseJWT(token)
	if expired {
		abort(oauth2.AccessDenied("expired access token"))
	} else if err != nil {
		abort(oauth2.AccessDenied("invalid access token"))
	}

	// get token id
	tokenID, err := coal.FromHex(claims.Id)
	if err != nil {
		abort(oauth2.AccessDenied("missing access token id"))
	}

	// get token
	accessToken := a.getToken(env, tokenID)
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

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) tokenEndpoint(env *environment) {
	// begin trace
	env.tracer.Push("flame/Authenticator.tokenEndpoint")

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

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// begin trace
	env.tracer.Push("flame/Authenticator.handleResourceOwnerPasswordCredentialsGrant")

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

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) handleClientCredentialsGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// begin trace
	env.tracer.Push("flame/Authenticator.handleClientCredentialsGrant")

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

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) handleRefreshTokenGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// begin trace
	env.tracer.Push("flame/Authenticator.handleRefreshTokenGrant")

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	claims, expired, err := a.policy.ParseJWT(req.RefreshToken)
	if expired {
		stack.Abort(oauth2.InvalidGrant("expired refresh token"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed refresh token"))
	}

	// get id
	id, err := coal.FromHex(claims.Id)
	if err != nil {
		stack.Abort(oauth2.InvalidRequest("invalid refresh token id"))
	}

	// get stored refresh token by signature
	rt := a.getToken(env, id)
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

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) handleAuthorizationCodeGrant(env *environment, req *oauth2.TokenRequest, client Client) {
	// begin trace
	env.tracer.Push("flame/Authenticator.handleAuthorizationCodeGrant")

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse authorization code
	claims, expired, err := a.policy.ParseJWT(req.Code)
	if expired {
		stack.Abort(oauth2.InvalidGrant("expired authorization code"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed authorization code"))
	}

	// get id
	id, err := coal.FromHex(claims.Id)
	if err != nil {
		stack.Abort(oauth2.InvalidRequest("invalid authorization code id"))
	}

	// TODO: We should revoke all descending tokens if a code is reused.

	// get stored authorization code by signature
	code := a.getToken(env, id)
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

	// validate redirect uri
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

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) revocationEndpoint(env *environment) {
	// begin trace
	env.tracer.Push("flame/Authenticator.revocationEndpoint")

	// parse authorization request
	req, err := revocation.ParseRequest(env.request)
	stack.AbortIf(err)

	// check token type hint
	if req.TokenTypeHint != "" && !revocation.KnownTokenType(req.TokenTypeHint) {
		stack.Abort(revocation.UnsupportedTokenType(""))
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
	claims, expired, err := a.policy.ParseJWT(req.Token)
	if expired {
		env.writer.WriteHeader(http.StatusOK)
		env.tracer.Pop()
		return
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// parse id
	id, err := coal.FromHex(claims.Id)
	stack.AbortIf(err)

	// get token
	token := a.getToken(env, id)
	if token != nil {
		// get data
		data := token.GetTokenData()

		// check ownership
		if data.ClientID != client.ID() {
			stack.Abort(oauth2.InvalidClient("wrong client"))
			return
		}

		// delete token
		a.deleteToken(env, id)
	}

	// write header
	env.writer.WriteHeader(http.StatusOK)

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) introspectionEndpoint(env *environment) {
	// begin trace
	env.tracer.Push("flame/Authenticator.introspectionEndpoint")

	// parse introspection request
	req, err := introspection.ParseRequest(env.request)
	stack.AbortIf(err)

	// check token type hint
	if req.TokenTypeHint != "" && !introspection.KnownTokenType(req.TokenTypeHint) {
		stack.Abort(introspection.UnsupportedTokenType(""))
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
	claims, expired, err := a.policy.ParseJWT(req.Token)
	if expired {
		stack.AbortIf(introspection.WriteResponse(env.writer, &introspection.Response{}))
		env.tracer.Pop()
		return
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// parse id
	id, err := coal.FromHex(claims.Id)
	stack.AbortIf(err)

	// prepare response
	res := &introspection.Response{}

	// get token
	token := a.getToken(env, id)
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
			res.TokenType = introspection.AccessToken
			if data.Type == RefreshToken {
				res.TokenType = introspection.RefreshToken
			}
			res.ExpiresAt = data.ExpiresAt.Unix()
			res.IssuedAt = token.ID().Timestamp().Unix()
			res.Identifier = token.ID().Hex()
			res.Extra = a.policy.TokenData(client, resourceOwner, token)
		}
	}

	// write response
	stack.AbortIf(introspection.WriteResponse(env.writer, res))

	// finish trace
	env.tracer.Pop()
}

func (a *Authenticator) issueTokens(env *environment, refreshable bool, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
	// begin trace
	env.tracer.Push("flame/Authenticator.issueTokens")

	// prepare expiration
	atExpiry := time.Now().Add(a.policy.AccessTokenLifespan)
	rtExpiry := time.Now().Add(a.policy.RefreshTokenLifespan)

	// save access token
	at := a.saveToken(env, AccessToken, scope, atExpiry, redirectURI, client, resourceOwner)

	// generate new access token
	atSignature, err := a.policy.GenerateJWT(at, client, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := bearer.NewTokenResponse(atSignature, int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	// issue a refresh token if requested
	if refreshable && env.grants.RefreshToken {
		// save refresh token
		rt := a.saveToken(env, RefreshToken, scope, rtExpiry, redirectURI, client, resourceOwner)

		// generate new refresh token
		rtSignature, err := a.policy.GenerateJWT(rt, client, resourceOwner)
		stack.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	// finish trace
	env.tracer.Pop()

	return res
}

func (a *Authenticator) issueCode(env *environment, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.CodeResponse {
	// begin trace
	env.tracer.Push("flame/Authenticator.issueCode")

	// prepare expiration
	expiry := time.Now().Add(a.policy.AuthorizationCodeLifespan)

	// save authorization code
	code := a.saveToken(env, AuthorizationCode, scope, expiry, redirectURI, client, resourceOwner)

	// generate new access token
	signature, err := a.policy.GenerateJWT(code, client, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := oauth2.NewCodeResponse(signature, redirectURI, "")

	// finish trace
	env.tracer.Pop()

	return res
}

func (a *Authenticator) findFirstClient(env *environment, id string) Client {
	// begin trace
	env.tracer.Push("flame/Authenticator.findFirstClient")

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.findClient(env, model, id)
		if c != nil {
			env.tracer.Pop()
			return c
		}
	}

	// finish trace
	env.tracer.Pop()

	return nil
}

func (a *Authenticator) findClient(env *environment, model Client, id string) Client {
	// begin trace
	env.tracer.Push("flame/Authenticator.findClient")

	// prepare client
	client := model.Meta().Make().(Client)

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
	err := a.store.TC(env.tracer, model).FindOne(nil, query).Decode(client)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize client
	coal.Init(client)

	// finish trace
	env.tracer.Pop()

	return client
}

func (a *Authenticator) getFirstClient(env *environment, id coal.ID) Client {
	// begin trace
	env.tracer.Push("flame/Authenticator.getFirstClient")

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.getClient(env, model, id)
		if c != nil {
			env.tracer.Pop()
			return c
		}
	}

	// finish trace
	env.tracer.Pop()

	return nil
}

func (a *Authenticator) getClient(env *environment, model Client, id coal.ID) Client {
	// begin trace
	env.tracer.Push("flame/Authenticator.getClient")

	// prepare client
	client := model.Meta().Make().(Client)

	// fetch client
	err := a.store.TC(env.tracer, model).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(client)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize client
	coal.Init(client)

	// finish trace
	env.tracer.Pop()

	return client
}

func (a *Authenticator) findFirstResourceOwner(env *environment, client Client, id string) ResourceOwner {
	// begin trace
	env.tracer.Push("flame/Authenticator.findFirstResourceOwner")

	// get resource owners
	resourceOwners, err := a.policy.ResourceOwners(client)
	stack.AbortIf(err)

	// check all available models in order
	for _, model := range resourceOwners {
		ro := a.findResourceOwner(env, client, model, id)
		if ro != nil {
			env.tracer.Pop()
			return ro
		}
	}

	// finish trace
	env.tracer.Pop()

	return nil
}

func (a *Authenticator) findResourceOwner(env *environment, client Client, model ResourceOwner, id string) ResourceOwner {
	// begin trace
	env.tracer.Push("flame/Authenticator.findResourceOwner")

	// prepare resource owner
	resourceOwner := coal.Init(model).Meta().Make().(ResourceOwner)

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
	err := a.store.TC(env.tracer, model).FindOne(nil, query).Decode(resourceOwner)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize resource owner
	coal.Init(resourceOwner)

	// finish trace
	env.tracer.Pop()

	return resourceOwner
}

func (a *Authenticator) getFirstResourceOwner(env *environment, client Client, id coal.ID) ResourceOwner {
	// begin trace
	env.tracer.Push("flame/Authenticator.getFirstResourceOwner")

	// get resource owners
	resourceOwners, err := a.policy.ResourceOwners(client)
	stack.AbortIf(err)

	// check all available models in order
	for _, model := range resourceOwners {
		ro := a.getResourceOwner(env, model, id)
		if ro != nil {
			env.tracer.Pop()
			return ro
		}
	}

	// finish trace
	env.tracer.Pop()

	return nil
}

func (a *Authenticator) getResourceOwner(env *environment, model ResourceOwner, id coal.ID) ResourceOwner {
	// begin trace
	env.tracer.Push("flame/Authenticator.getResourceOwner")

	// prepare object
	resourceOwner := coal.Init(model).Meta().Make().(ResourceOwner)

	// fetch resource owner
	err := a.store.TC(env.tracer, model).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(resourceOwner)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize resource owner
	coal.Init(resourceOwner)

	// finish trace
	env.tracer.Pop()

	return resourceOwner
}

func (a *Authenticator) getToken(env *environment, id coal.ID) GenericToken {
	// begin trace
	env.tracer.Push("flame/Authenticator.getToken")

	// prepare object
	obj := a.policy.Token.Meta().Make()

	// fetch token
	err := a.store.TC(env.tracer, obj).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(obj)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize token
	token := coal.Init(obj).(GenericToken)

	// finish trace
	env.tracer.Pop()

	return token
}

func (a *Authenticator) saveToken(env *environment, typ TokenType, scope []string, expiresAt time.Time, redirectURI string, client Client, resourceOwner ResourceOwner) GenericToken {
	// begin trace
	env.tracer.Push("flame/Authenticator.saveToken")

	// prepare token
	token := a.policy.Token.Meta().Make().(GenericToken)

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
	_, err := a.store.TC(env.tracer, token).InsertOne(nil, token)
	stack.AbortIf(err)

	// finish trace
	env.tracer.Pop()

	return token
}

func (a *Authenticator) deleteToken(env *environment, id coal.ID) {
	// begin trace
	env.tracer.Push("flame/Authenticator.deleteToken")

	// delete token
	_, err := a.store.TC(env.tracer, a.policy.Token).DeleteOne(nil, bson.M{
		"_id": id,
	})
	if err == mongo.ErrNoDocuments {
		err = nil
	}
	stack.AbortIf(err)

	// finish trace
	env.tracer.Pop()
}
