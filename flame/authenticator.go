// Package flame implements an authenticator that provides OAuth2 compatible
// authentication with JWT tokens.
package flame

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/xo"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
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
		// create tracer
		tracer, rcx := xo.CreateTracer(r.Context(), "flame/Authenticator.Endpoint")
		tracer.Tag("prefix", prefix)
		defer tracer.End()
		r = r.WithContext(rcx)

		// continue any previous aborts
		defer xo.Resume(func(err error) {
			// directly write oauth2 errors
			var oauth2Error *oauth2.Error
			if errors.As(err, &oauth2Error) {
				_ = oauth2.WriteError(w, oauth2Error)
				return
			}

			// record error
			tracer.Record(err)

			// otherwise, report critical errors
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

		// prepare context
		ctx := &Context{
			Context: rcx,
			Request: r,
			writer:  w,
			Tracer:  tracer,
		}

		// call endpoints
		switch s[0] {
		case "authorize":
			a.authorizationEndpoint(ctx)
		case "token":
			a.tokenEndpoint(ctx)
		case "revoke":
			a.revocationEndpoint(ctx)
		case "introspect":
			a.introspectionEndpoint(ctx)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

// Authorizer returns a middleware that can be used to authorize a request by
// requiring an access token with the provided scope to be granted.
func (a *Authenticator) Authorizer(scope []string, force, loadClient, loadResourceOwner bool) func(http.Handler) http.Handler {
	// get scope str
	scopeStr := oauth2.Scope(scope).String()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// create tracer
			tracer, rcx := xo.CreateTracer(r.Context(), "flame/Authenticator.Authorizer")
			tracer.Tag("scope", scopeStr)
			tracer.Tag("force", force)
			tracer.Tag("loadClient", loadClient)
			tracer.Tag("loadResourceOwner", loadResourceOwner)
			defer tracer.End()
			r = r.WithContext(rcx)

			// immediately pass on request if force is not set and there is
			// no authentication information provided
			if !force && r.Header.Get("Authorization") == "" {
				// call next handler
				next.ServeHTTP(w, r)

				return
			}

			// continue any previous aborts
			defer xo.Resume(func(err error) {
				// directly write bearer errors
				var bearerError *oauth2.Error
				if errors.As(err, &bearerError) {
					_ = oauth2.WriteBearerError(w, bearerError)
					return
				}

				// record error
				tracer.Record(err)

				// otherwise, report critical errors
				if a.reporter != nil {
					a.reporter(err)
				}

				// write generic server error
				_ = oauth2.WriteBearerError(w, oauth2.ServerError(""))
			})

			// parse bearer token
			tk, err := oauth2.ParseBearerToken(r)
			xo.AbortIf(err)

			// parse token
			key, err := a.policy.Verify(rcx, tk)
			if heat.ErrExpiredToken.Is(err) {
				xo.Abort(oauth2.InvalidToken("expired bearer token"))
			} else if err != nil {
				xo.Abort(oauth2.InvalidToken("malformed bearer token"))
			}

			// prepare context
			ctx := &Context{
				Context: rcx,
				Request: r,
				writer:  w,
				Tracer:  tracer,
			}

			// get token
			accessToken := a.getToken(ctx, key.Base.ID)
			if accessToken == nil {
				xo.Abort(oauth2.InvalidToken("unknown bearer token"))
			}

			// get token data
			data := accessToken.GetTokenData()

			// validate token type
			if data.Type != AccessToken {
				xo.Abort(oauth2.InvalidToken("invalid bearer token type"))
			}

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				xo.Abort(oauth2.InvalidToken("expired access token"))
			}

			// validate scope
			if !data.Scope.Includes(scope) {
				xo.Abort(oauth2.InsufficientScope(scope))
			}

			// create new context with access token
			rcx = context.WithValue(rcx, AccessTokenContextKey, accessToken)

			// call next handler if client should not be loaded
			if !loadClient {
				// call next handler
				next.ServeHTTP(w, r.WithContext(rcx))

				return
			}

			// get client
			client := a.getFirstClient(ctx, data.ClientID)
			if client == nil {
				xo.Abort(xo.F("missing client"))
			}

			// create new context with client
			rcx = context.WithValue(rcx, ClientContextKey, client)

			// call next handler if resource owner does not exist or should not
			// be loaded
			if data.ResourceOwnerID == nil || !loadResourceOwner {
				// call next handler
				next.ServeHTTP(w, r.WithContext(rcx))

				return
			}

			// get resource owner
			resourceOwner := a.getFirstResourceOwner(ctx, client, *data.ResourceOwnerID)
			if resourceOwner == nil {
				xo.Abort(oauth2.InvalidToken("missing resource owner"))
			}

			// create new context with resource owner
			rcx = context.WithValue(rcx, ResourceOwnerContextKey, resourceOwner)

			// call next handler
			next.ServeHTTP(w, r.WithContext(rcx))
		})
	}
}

func (a *Authenticator) authorizationEndpoint(ctx *Context) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.authorizationEndpoint")
	defer ctx.Tracer.Pop()

	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(ctx.Request)
	xo.AbortIf(err)

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		xo.Abort(oauth2.InvalidRequest("unknown response type"))
	}

	// get client
	client := a.findFirstClient(ctx, req.ClientID)
	if client == nil {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate redirect URI
	req.RedirectURI, err = a.policy.RedirectURIValidator(ctx, client, req.RedirectURI)
	if ErrInvalidRedirectURI.Is(err) {
		xo.Abort(oauth2.InvalidRequest("invalid redirect uri"))
	} else if err != nil {
		xo.Abort(err)
	}

	// get grants
	ctx.grants, err = a.policy.Grants(ctx, client)
	xo.AbortIf(err)

	/* client is valid */

	// validate response type
	if req.ResponseType == oauth2.TokenResponseType && !ctx.grants.Implicit {
		xo.Abort(oauth2.UnsupportedResponseType(""))
	} else if req.ResponseType == oauth2.CodeResponseType && !ctx.grants.AuthorizationCode {
		xo.Abort(oauth2.UnsupportedResponseType(""))
	}

	// prepare abort method
	abort := func(err *oauth2.Error) {
		xo.Abort(err.SetRedirect(req.RedirectURI, req.State, req.ResponseType == oauth2.TokenResponseType))
	}

	// check request method
	if ctx.Request.Method == "GET" {
		// get approval url
		url, err := a.policy.ApprovalURL(ctx, client)
		if err != nil {
			xo.Abort(err)
		} else if url == "" {
			abort(oauth2.InvalidRequest("unsupported request method"))
		}

		// prepare params
		params := map[string]string{}
		for name, values := range ctx.Request.URL.Query() {
			params[name] = values[0]
		}

		// perform redirect
		xo.AbortIf(oauth2.WriteRedirect(ctx.writer, url, params, false))

		return
	}

	// get access token
	token := ctx.Request.Form.Get("access_token")
	if token == "" {
		abort(oauth2.AccessDenied("missing access token"))
	}

	// parse token
	key, err := a.policy.Verify(ctx, token)
	if heat.ErrExpiredToken.Is(err) {
		abort(oauth2.AccessDenied("expired access token"))
	} else if err != nil {
		abort(oauth2.AccessDenied("invalid access token"))
	}

	// get token
	accessToken := a.getToken(ctx, key.Base.ID)
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
	resourceOwner := a.getFirstResourceOwner(ctx, client, *data.ResourceOwnerID)
	if resourceOwner == nil {
		abort(oauth2.AccessDenied("unknown resource owner"))
	}

	// validate & grant scope
	scope, err := a.policy.ApproveStrategy(ctx, client, resourceOwner, accessToken, req.Scope)
	if ErrApprovalRejected.Is(err) {
		abort(oauth2.AccessDenied("approval rejected"))
	} else if ErrInvalidScope.Is(err) {
		abort(oauth2.InvalidScope(""))
	} else if err != nil {
		xo.Abort(err)
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		// issue access token
		res := a.issueTokens(ctx, false, scope, req.RedirectURI, client, resourceOwner)
		res.SetRedirect(req.RedirectURI, req.State)

		// write response
		xo.AbortIf(oauth2.WriteTokenResponse(ctx.writer, res))
	case oauth2.CodeResponseType:
		// issue authorization code
		res := a.issueCode(ctx, scope, req.RedirectURI, client, resourceOwner)
		res.State = req.State

		// write response
		xo.AbortIf(oauth2.WriteCodeResponse(ctx.writer, res))
	}
}

func (a *Authenticator) tokenEndpoint(ctx *Context) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.tokenEndpoint")
	defer ctx.Tracer.Pop()

	// parse token request
	req, err := oauth2.ParseTokenRequest(ctx.Request)
	xo.AbortIf(err)

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		xo.Abort(oauth2.InvalidRequest("unknown grant type"))
	}

	// get client
	client := a.findFirstClient(ctx, req.ClientID)
	if client == nil {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// get grants
	ctx.grants, err = a.policy.Grants(ctx, client)
	xo.AbortIf(err)

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		// check availability
		if !ctx.grants.Password {
			xo.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle resource owner password credentials grant
		a.handleResourceOwnerPasswordCredentialsGrant(ctx, req, client)
	case oauth2.ClientCredentialsGrantType:
		// check availability
		if !ctx.grants.ClientCredentials {
			xo.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle client credentials grant
		a.handleClientCredentialsGrant(ctx, req, client)
	case oauth2.RefreshTokenGrantType:
		// check availability
		if !ctx.grants.RefreshToken {
			xo.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle refresh token grant
		a.handleRefreshTokenGrant(ctx, req, client)
	case oauth2.AuthorizationCodeGrantType:
		// check availability
		if !ctx.grants.AuthorizationCode {
			xo.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle authorization code grant
		a.handleAuthorizationCodeGrant(ctx, req, client)
	}
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(ctx *Context, req *oauth2.TokenRequest, client Client) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.handleResourceOwnerPasswordCredentialsGrant")
	defer ctx.Tracer.Pop()

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// get resource owner
	resourceOwner := a.findFirstResourceOwner(ctx, client, req.Username)
	if resourceOwner == nil {
		xo.Abort(oauth2.AccessDenied("")) // never expose reason!
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		xo.Abort(oauth2.AccessDenied("")) // never expose reason!
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(ctx, client, resourceOwner, req.Scope)
	if ErrGrantRejected.Is(err) {
		xo.Abort(oauth2.AccessDenied("")) // never expose reason!
	} else if ErrInvalidScope.Is(err) {
		xo.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		xo.Abort(err)
	}

	// issue access token
	res := a.issueTokens(ctx, true, scope, "", client, resourceOwner)

	// write response
	xo.AbortIf(oauth2.WriteTokenResponse(ctx.writer, res))
}

func (a *Authenticator) handleClientCredentialsGrant(ctx *Context, req *oauth2.TokenRequest, client Client) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.handleClientCredentialsGrant")
	defer ctx.Tracer.Pop()

	// check confidentiality
	if !client.IsConfidential() {
		xo.Abort(oauth2.InvalidClient("non confidential client"))
	}

	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(ctx, client, nil, req.Scope)
	if ErrGrantRejected.Is(err) {
		xo.Abort(oauth2.AccessDenied("grant rejected"))
	} else if ErrInvalidScope.Is(err) {
		xo.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		xo.Abort(err)
	}

	// issue access token
	res := a.issueTokens(ctx, true, scope, "", client, nil)

	// write response
	xo.AbortIf(oauth2.WriteTokenResponse(ctx.writer, res))
}

func (a *Authenticator) handleRefreshTokenGrant(ctx *Context, req *oauth2.TokenRequest, client Client) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.handleRefreshTokenGrant")
	defer ctx.Tracer.Pop()

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	key, err := a.policy.Verify(ctx, req.RefreshToken)
	if heat.ErrExpiredToken.Is(err) {
		xo.Abort(oauth2.InvalidGrant("expired refresh token"))
	} else if err != nil {
		xo.Abort(oauth2.InvalidRequest("malformed refresh token"))
	}

	// get stored refresh token by signature
	rt := a.getToken(ctx, key.Base.ID)
	if rt == nil {
		xo.Abort(oauth2.InvalidGrant("unknown refresh token"))
	}

	// get token data
	data := rt.GetTokenData()

	// validate type
	if data.Type != RefreshToken {
		xo.Abort(oauth2.InvalidGrant("invalid refresh token type"))
	}

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		xo.Abort(oauth2.InvalidGrant("expired refresh token"))
	}

	// validate ownership
	if data.ClientID != client.ID() {
		xo.Abort(oauth2.InvalidGrant("invalid refresh token ownership"))
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !data.Scope.Includes(req.Scope) {
		xo.Abort(oauth2.InvalidScope("scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = a.getFirstResourceOwner(ctx, client, *data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(ctx, true, req.Scope, data.RedirectURI, client, ro)

	// delete refresh token
	a.deleteToken(ctx, rt.ID())

	// write response
	xo.AbortIf(oauth2.WriteTokenResponse(ctx.writer, res))
}

func (a *Authenticator) handleAuthorizationCodeGrant(ctx *Context, req *oauth2.TokenRequest, client Client) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.handleAuthorizationCodeGrant")
	defer ctx.Tracer.Pop()

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse authorization code
	key, err := a.policy.Verify(ctx, req.Code)
	if heat.ErrExpiredToken.Is(err) {
		xo.Abort(oauth2.InvalidGrant("expired authorization code"))
	} else if err != nil {
		xo.Abort(oauth2.InvalidRequest("malformed authorization code"))
	}

	// TODO: We should revoke all descending tokens if a code is reused.

	// get stored authorization code by signature
	code := a.getToken(ctx, key.Base.ID)

	if code == nil {
		xo.Abort(oauth2.InvalidGrant("unknown authorization code"))
	}

	// get token data
	data := code.GetTokenData()

	// validate type
	if data.Type != AuthorizationCode {
		xo.Abort(oauth2.InvalidGrant("invalid authorization code type"))
	}

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		xo.Abort(oauth2.InvalidGrant("expired authorization code"))
	}

	// validate ownership
	if data.ClientID != client.ID() {
		xo.Abort(oauth2.InvalidGrant("invalid authorization code ownership"))
	}

	// validate redirect URI
	req.RedirectURI, err = a.policy.RedirectURIValidator(ctx, client, req.RedirectURI)
	if ErrInvalidRedirectURI.Is(err) {
		xo.Abort(oauth2.InvalidRequest("invalid redirect uri"))
	} else if err != nil {
		xo.Abort(err)
	}

	// compare redirect URIs
	if data.RedirectURI != req.RedirectURI {
		xo.Abort(oauth2.InvalidGrant("redirect uri mismatch"))
	}

	// inherit scope from stored authorization code
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !data.Scope.Includes(req.Scope) {
		xo.Abort(oauth2.InvalidScope("scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = a.getFirstResourceOwner(ctx, client, *data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(ctx, true, req.Scope, data.RedirectURI, client, ro)

	// delete authorization code
	a.deleteToken(ctx, code.ID())

	// write response
	xo.AbortIf(oauth2.WriteTokenResponse(ctx.writer, res))
}

func (a *Authenticator) revocationEndpoint(ctx *Context) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.revocationEndpoint")
	defer ctx.Tracer.Pop()

	// parse authorization request
	req, err := oauth2.ParseRevocationRequest(ctx.Request)
	xo.AbortIf(err)

	// check token type hint
	if req.TokenTypeHint != "" && !oauth2.KnownTokenType(req.TokenTypeHint) {
		xo.Abort(oauth2.UnsupportedTokenType(""))
	}

	// get client
	client := a.findFirstClient(ctx, req.ClientID)
	if client == nil {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	key, err := a.policy.Verify(ctx, req.Token)
	if heat.ErrExpiredToken.Is(err) {
		ctx.writer.WriteHeader(http.StatusOK)
		return
	} else if err != nil {
		xo.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// get token
	token := a.getToken(ctx, key.Base.ID)
	if token != nil {
		// get data
		data := token.GetTokenData()

		// check ownership
		if data.ClientID != client.ID() {
			xo.Abort(oauth2.InvalidClient("wrong client"))
			return
		}

		// delete token
		a.deleteToken(ctx, key.Base.ID)
	}

	// write header
	ctx.writer.WriteHeader(http.StatusOK)
}

func (a *Authenticator) introspectionEndpoint(ctx *Context) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.introspectionEndpoint")
	defer ctx.Tracer.Pop()

	// parse introspection request
	req, err := oauth2.ParseIntrospectionRequest(ctx.Request)
	xo.AbortIf(err)

	// check token type hint
	if req.TokenTypeHint != "" && !oauth2.KnownTokenType(req.TokenTypeHint) {
		xo.Abort(oauth2.UnsupportedTokenType(""))
	}

	// get client
	client := a.findFirstClient(ctx, req.ClientID)
	if client == nil {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// authenticate client if confidential
	if client.IsConfidential() && !client.ValidSecret(req.ClientSecret) {
		xo.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	key, err := a.policy.Verify(ctx, req.Token)
	if heat.ErrExpiredToken.Is(err) {
		xo.AbortIf(oauth2.WriteIntrospectionResponse(ctx.writer, &oauth2.IntrospectionResponse{}))
		return
	} else if err != nil {
		xo.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// prepare response
	res := &oauth2.IntrospectionResponse{}

	// get token
	token := a.getToken(ctx, key.Base.ID)
	if token != nil {
		// get data
		data := token.GetTokenData()

		// check ownership
		if data.ClientID != client.ID() {
			xo.Abort(oauth2.InvalidClient("wrong client"))
			return
		}

		// get resource owner
		var resourceOwner ResourceOwner
		if data.ResourceOwnerID != nil {
			resourceOwner = a.getFirstResourceOwner(ctx, client, *data.ResourceOwnerID)
		}

		// get validity
		expired := data.ExpiresAt.Before(time.Now())

		// set response if valid and can be introspected
		if !expired && (data.Type == AccessToken || data.Type == RefreshToken) {
			res.Active = true
			res.Scope = data.Scope
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
	xo.AbortIf(oauth2.WriteIntrospectionResponse(ctx.writer, res))
}

func (a *Authenticator) issueTokens(ctx *Context, refreshable bool, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
	// trace
	ctx.Tracer.Push("flame/Authenticator.issueTokens")
	defer ctx.Tracer.Pop()

	// prepare expiration
	atExpiry := time.Now().Add(a.policy.AccessTokenLifespan)
	rtExpiry := time.Now().Add(a.policy.RefreshTokenLifespan)

	// save access token
	at := a.saveToken(ctx, AccessToken, scope, atExpiry, redirectURI, client, resourceOwner)

	// generate new access token
	atSignature, err := a.policy.Issue(ctx, at, client, resourceOwner)
	xo.AbortIf(err)

	// prepare response
	res := oauth2.NewBearerTokenResponse(atSignature, int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	// issue a refresh token if requested
	if refreshable && ctx.grants.RefreshToken {
		// save refresh token
		rt := a.saveToken(ctx, RefreshToken, scope, rtExpiry, redirectURI, client, resourceOwner)

		// generate new refresh token
		rtSignature, err := a.policy.Issue(ctx, rt, client, resourceOwner)
		xo.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	return res
}

func (a *Authenticator) issueCode(ctx *Context, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.CodeResponse {
	// trace
	ctx.Tracer.Push("flame/Authenticator.issueCode")
	defer ctx.Tracer.Pop()

	// prepare expiration
	expiry := time.Now().Add(a.policy.AuthorizationCodeLifespan)

	// save authorization code
	code := a.saveToken(ctx, AuthorizationCode, scope, expiry, redirectURI, client, resourceOwner)

	// generate new access token
	signature, err := a.policy.Issue(ctx, code, client, resourceOwner)
	xo.AbortIf(err)

	// prepare response
	res := oauth2.NewCodeResponse(signature, redirectURI, "")

	return res
}

func (a *Authenticator) findFirstClient(ctx *Context, id string) Client {
	// trace
	ctx.Tracer.Push("flame/Authenticator.findFirstClient")
	defer ctx.Tracer.Pop()

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.findClient(ctx, model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) findClient(ctx *Context, model Client, id string) Client {
	// trace
	ctx.Tracer.Push("flame/Authenticator.findClient")
	defer ctx.Tracer.Pop()

	// prepare client
	client := coal.GetMeta(model).Make().(Client)

	// use tagged field if present
	var filters []bson.M
	idField := coal.L(model, "flame-client-id", false)
	if idField != "" {
		filters = []bson.M{
			{idField: id},
		}
	} else if coal.IsHex(id) {
		filters = []bson.M{
			{"_id": coal.MustFromHex(id)},
		}
	} else {
		xo.Abort(xo.F("unable to determine client id field"))
	}

	// add additional filter if provided
	if a.policy.ClientFilter != nil {
		// run filter function
		filter, err := a.policy.ClientFilter(ctx, model)
		if ErrInvalidFilter.Is(err) {
			xo.Abort(oauth2.InvalidRequest("invalid filter"))
		} else if err != nil {
			xo.Abort(err)
		}

		// add filter if present
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	// fetch client
	found, err := a.store.M(model).FindFirst(ctx, client, bson.M{
		"$and": filters,
	}, nil, 0, false)
	xo.AbortIf(err)
	if !found {
		return nil
	}

	return client
}

func (a *Authenticator) getFirstClient(ctx *Context, id coal.ID) Client {
	// trace
	ctx.Tracer.Push("flame/Authenticator.getFirstClient")
	defer ctx.Tracer.Pop()

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.getClient(ctx, model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) getClient(ctx *Context, model Client, id coal.ID) Client {
	// trace
	ctx.Tracer.Push("flame/Authenticator.getClient")
	defer ctx.Tracer.Pop()

	// prepare client
	client := coal.GetMeta(model).Make().(Client)

	// fetch client
	found, err := a.store.M(model).Find(ctx, client, id, false)
	xo.AbortIf(err)
	if !found {
		return nil
	}

	return client
}

func (a *Authenticator) findFirstResourceOwner(ctx *Context, client Client, id string) ResourceOwner {
	// trace
	ctx.Tracer.Push("flame/Authenticator.findFirstResourceOwner")
	defer ctx.Tracer.Pop()

	// get resource owners
	resourceOwners, err := a.policy.ResourceOwners(ctx, client)
	xo.AbortIf(err)

	// check all available models in order
	for _, model := range resourceOwners {
		ro := a.findResourceOwner(ctx, client, model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) findResourceOwner(ctx *Context, client Client, model ResourceOwner, id string) ResourceOwner {
	// trace
	ctx.Tracer.Push("flame/Authenticator.findResourceOwner")
	defer ctx.Tracer.Pop()

	// prepare resource owner
	resourceOwner := coal.GetMeta(model).Make().(ResourceOwner)

	// use tagged field if present
	var filters []bson.M
	idField := coal.L(model, "flame-resource-owner-id", false)
	if idField != "" {
		filters = []bson.M{
			{idField: id},
		}
	} else if coal.IsHex(id) {
		filters = []bson.M{
			{"_id": coal.MustFromHex(id)},
		}
	} else {
		xo.Abort(xo.F("unable to determine resource owner id field"))
	}

	// add additional filter if provided
	if a.policy.ResourceOwnerFilter != nil {
		// run filter function
		filter, err := a.policy.ResourceOwnerFilter(ctx, client, model)
		if ErrInvalidFilter.Is(err) {
			xo.Abort(oauth2.InvalidRequest("invalid filter"))
		} else if err != nil {
			xo.Abort(err)
		}

		// add filter if present
		if filter != nil {
			filters = append(filters, filter)
		}
	}

	// fetch resource owner
	found, err := a.store.M(model).FindFirst(ctx, resourceOwner, bson.M{
		"$and": filters,
	}, nil, 0, false)
	xo.AbortIf(err)
	if !found {
		return nil
	}

	return resourceOwner
}

func (a *Authenticator) getFirstResourceOwner(ctx *Context, client Client, id coal.ID) ResourceOwner {
	// trace
	ctx.Tracer.Push("flame/Authenticator.getFirstResourceOwner")
	defer ctx.Tracer.Pop()

	// get resource owners
	resourceOwners, err := a.policy.ResourceOwners(ctx, client)
	xo.AbortIf(err)

	// check all available models in order
	for _, model := range resourceOwners {
		ro := a.getResourceOwner(ctx, model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) getResourceOwner(ctx *Context, model ResourceOwner, id coal.ID) ResourceOwner {
	// trace
	ctx.Tracer.Push("flame/Authenticator.getResourceOwner")
	defer ctx.Tracer.Pop()

	// prepare object
	resourceOwner := coal.GetMeta(model).Make().(ResourceOwner)

	// fetch resource owner
	found, err := a.store.M(model).Find(ctx, resourceOwner, id, false)
	xo.AbortIf(err)
	if !found {
		return nil
	}

	return resourceOwner
}

func (a *Authenticator) getToken(ctx *Context, id coal.ID) GenericToken {
	// trace
	ctx.Tracer.Push("flame/Authenticator.getToken")
	defer ctx.Tracer.Pop()

	// prepare object
	token := coal.GetMeta(a.policy.Token).Make().(GenericToken)

	// fetch token
	found, err := a.store.M(token).Find(ctx, token, id, false)
	xo.AbortIf(err)
	if !found {
		return nil
	}

	return token
}

func (a *Authenticator) saveToken(ctx *Context, typ TokenType, scope []string, expiresAt time.Time, redirectURI string, client Client, resourceOwner ResourceOwner) GenericToken {
	// trace
	ctx.Tracer.Push("flame/Authenticator.saveToken")
	defer ctx.Tracer.Pop()

	// create token with id
	token := coal.GetMeta(a.policy.Token).Make().(GenericToken)
	token.GetBase().DocID = coal.New()

	// get resource owner id
	var roID *coal.ID
	if resourceOwner != nil {
		roID = stick.P(resourceOwner.ID())
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
	xo.AbortIf(a.store.M(token).Insert(ctx, token))

	return token
}

func (a *Authenticator) deleteToken(ctx *Context, id coal.ID) {
	// trace
	ctx.Tracer.Push("flame/Authenticator.deleteToken")
	defer ctx.Tracer.Pop()

	// delete token
	_, err := a.store.M(a.policy.Token).Delete(ctx, nil, id)
	xo.AbortIf(err)
}
