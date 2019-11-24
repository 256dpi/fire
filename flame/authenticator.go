// Package flame implements an authenticator that provides OAuth2 compatible
// authentication with JWT tokens.
package flame

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/bearer"
	"github.com/256dpi/oauth2/revocation"
	"github.com/256dpi/stack"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

type ctxKey string

const (
	// AccessTokenContextKey is the key used to save the access token in a context.
	AccessTokenContextKey = ctxKey("access-token")

	// ClientContextKey is the key used to save the client in a context.
	ClientContextKey = ctxKey("client")

	// ResourceOwnerContextKey is the key used to save the resource owner in a context.
	ResourceOwnerContextKey = ctxKey("resource-owner")
)

type state struct {
	request *http.Request
	writer  http.ResponseWriter
	store   *coal.Store
	tracer  *fire.Tracer
}

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant.
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
		if len(s) != 1 || (s[0] != "authorize" && s[0] != "token" && s[0] != "revoke") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// create
		state := &state{
			request: r,
			writer:  w,
			store:   a.store,
			tracer:  tracer,
		}

		// call endpoints
		switch s[0] {
		case "authorize":
			a.authorizationEndpoint(state)
		case "token":
			a.tokenEndpoint(state)
		case "revoke":
			a.revocationEndpoint(state)
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
			s := oauth2.ParseScope(scope)

			// parse bearer token
			tk, err := bearer.ParseToken(r)
			stack.AbortIf(err)

			// parse token
			claims, expired, err := a.policy.ParseJWT(tk)
			if expired {
				stack.Abort(bearer.InvalidToken("expired token"))
			} else if err != nil {
				stack.Abort(bearer.InvalidToken("malformed token"))
			}

			// create state
			state := &state{
				request: r,
				writer:  w,
				store:   a.store,
				tracer:  tracer,
			}

			// get id
			id, err := coal.FromHex(claims.Id)
			if err != nil {
				stack.Abort(bearer.InvalidToken("invalid id"))
			}

			// get token
			accessToken := a.getToken(state, a.policy.Token, id)
			if accessToken == nil {
				stack.Abort(bearer.InvalidToken("unknown token"))
			}

			// get token data
			data := accessToken.GetTokenData()

			// validate token type
			if data.Type != AccessToken {
				stack.Abort(bearer.InvalidToken("invalid type"))
			}

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				stack.Abort(bearer.InvalidToken("expired token"))
			}

			// validate scope
			if !oauth2.Scope(data.Scope).Includes(s) {
				stack.Abort(bearer.InsufficientScope(s.String()))
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
			client := a.getFirstClient(state, data.ClientID)
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
			resourceOwner := a.getFirstResourceOwner(state, client, *data.ResourceOwnerID)
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

func (a *Authenticator) authorizationEndpoint(state *state) {
	// begin trace
	state.tracer.Push("flame/Authenticator.authorizationEndpoint")

	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(state.request)
	stack.AbortIf(err)

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		stack.Abort(oauth2.InvalidRequest("unknown response type"))
	}

	// get client
	client := a.findFirstClient(state, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate redirect url
	if !client.ValidRedirectURL(req.RedirectURI) {
		stack.Abort(oauth2.InvalidRequest("invalid redirect url"))
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		// check availability
		if !a.policy.ImplicitGrant {
			stack.Abort(oauth2.UnsupportedResponseType(""))
		}

		// handle implicit grant
		a.handleImplicitGrant(state, req, client)
	case oauth2.CodeResponseType:
		// check availability
		if !a.policy.AuthorizationCodeGrant {
			stack.Abort(oauth2.UnsupportedResponseType(""))
		}

		// Handle authorization code grant
		a.handleAuthorizationCodeGrantAuthorization(state, req, client)
	}

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) handleImplicitGrant(state *state, req *oauth2.AuthorizationRequest, client Client) {
	// begin trace
	state.tracer.Push("flame/Authenticator.handleImplicitGrant")

	// check request method
	if state.request.Method == "GET" {
		stack.Abort(oauth2.InvalidRequest("unallowed request method").SetRedirect(req.RedirectURI, req.State, true))
	}

	// TODO: Optionally force redirect to frontend if no credentials have been provided.
	//  => Maybe we als check the existence of a random request token?

	// get credentials
	username := state.request.PostForm.Get("username")
	password := state.request.PostForm.Get("password")

	// get resource owner
	resourceOwner := a.findFirstResourceOwner(state, client, username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(req.Scope, client, resourceOwner)
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope("").SetRedirect(req.RedirectURI, req.State, true))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(state, false, scope, req.RedirectURI, client, resourceOwner)

	// redirect response
	res.SetRedirect(req.RedirectURI, req.State, true)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) handleAuthorizationCodeGrantAuthorization(state *state, req *oauth2.AuthorizationRequest, client Client) {
	// begin trace
	state.tracer.Push("flame/Authenticator.handleAuthorizationCodeGrantAuthorization")

	// check request method
	if state.request.Method == "GET" {
		stack.Abort(oauth2.InvalidRequest("unallowed request method").SetRedirect(req.RedirectURI, req.State, false))
	}

	// TODO: Optionally force redirect to frontend if no credentials have been provided.
	//  => Maybe we als check the existence of a random request token?

	// get credentials
	username := state.request.PostForm.Get("username")
	password := state.request.PostForm.Get("password")

	// get resource owner
	resourceOwner := a.findFirstResourceOwner(state, client, username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, false))
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, false))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(req.Scope, client, resourceOwner)
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, false))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope("").SetRedirect(req.RedirectURI, req.State, false))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue code
	res := a.issueCode(state, scope, req.RedirectURI, client, resourceOwner)

	// set state and redirect uri
	res.State = req.State
	res.RedirectURI = req.RedirectURI

	// write response
	stack.AbortIf(oauth2.WriteCodeResponse(state.writer, res))

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) tokenEndpoint(state *state) {
	// begin trace
	state.tracer.Push("flame/Authenticator.tokenEndpoint")

	// parse token request
	req, err := oauth2.ParseTokenRequest(state.request)
	stack.AbortIf(err)

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		stack.Abort(oauth2.InvalidRequest("unknown grant type"))
	}

	// get client
	client := a.findFirstClient(state, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		// check availability
		if !a.policy.PasswordGrant {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle resource owner password credentials grant
		a.handleResourceOwnerPasswordCredentialsGrant(state, req, client)
	case oauth2.ClientCredentialsGrantType:
		// check availability
		if !a.policy.ClientCredentialsGrant {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle client credentials grant
		a.handleClientCredentialsGrant(state, req, client)
	case oauth2.RefreshTokenGrantType:
		// handle refresh token grant
		a.handleRefreshTokenGrant(state, req, client)
	case oauth2.AuthorizationCodeGrantType:
		// check availability
		if !a.policy.AuthorizationCodeGrant {
			stack.Abort(oauth2.UnsupportedGrantType(""))
		}

		// handle authorization code grant
		a.handleAuthorizationCodeGrant(state, req, client)
	}

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(state *state, req *oauth2.TokenRequest, client Client) {
	// begin trace
	state.tracer.Push("flame/Authenticator.handleResourceOwnerPasswordCredentialsGrant")

	// get resource owner
	resourceOwner := a.findFirstResourceOwner(state, client, req.Username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(req.Scope, client, resourceOwner)
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied(""))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(state, true, scope, "", client, resourceOwner)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) handleClientCredentialsGrant(state *state, req *oauth2.TokenRequest, client Client) {
	// begin trace
	state.tracer.Push("flame/Authenticator.handleClientCredentialsGrant")

	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(req.Scope, client, nil)
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied(""))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(state, true, scope, "", client, nil)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) handleRefreshTokenGrant(state *state, req *oauth2.TokenRequest, client Client) {
	// begin trace
	state.tracer.Push("flame/Authenticator.handleRefreshTokenGrant")

	// parse token
	claims, expired, err := a.policy.ParseJWT(req.RefreshToken)
	if expired {
		stack.Abort(oauth2.InvalidGrant("expired refresh token"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// get id
	id, err := coal.FromHex(claims.Id)
	if err != nil {
		stack.Abort(oauth2.InvalidRequest("invalid id"))
	}

	// get stored refresh token by signature
	rt := a.getToken(state, a.policy.Token, id)
	if rt == nil {
		stack.Abort(oauth2.InvalidGrant("unknown refresh token"))
	}

	// get token data
	data := rt.GetTokenData()

	// validate type
	if data.Type != RefreshToken {
		stack.Abort(oauth2.InvalidGrant("invalid type"))
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
	if !oauth2.Scope(data.Scope).Includes(req.Scope) {
		stack.Abort(oauth2.InvalidScope("scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = a.getFirstResourceOwner(state, client, *data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(state, true, req.Scope, data.RedirectURI, client, ro)

	// delete refresh token
	a.deleteToken(state, a.policy.Token, rt.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) handleAuthorizationCodeGrant(state *state, req *oauth2.TokenRequest, client Client) {
	// begin trace
	state.tracer.Push("flame/Authenticator.handleAuthorizationCodeGrant")

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
		stack.Abort(oauth2.InvalidRequest("invalid id"))
	}

	// get stored authorization code by signature
	code := a.getToken(state, a.policy.Token, id)
	if code == nil {
		stack.Abort(oauth2.InvalidGrant("unknown authorization code"))
	}

	// get token data
	data := code.GetTokenData()

	// validate type
	if data.Type != AuthorizationCode {
		stack.Abort(oauth2.InvalidGrant("invalid type"))
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
		stack.Abort(oauth2.InvalidGrant("changed redirect uri"))
	}

	// inherit scope from stored authorization code
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !oauth2.Scope(data.Scope).Includes(req.Scope) {
		stack.Abort(oauth2.InvalidScope("scope exceeds the originally granted scope"))
	}

	// get resource owner
	var ro ResourceOwner
	if data.ResourceOwnerID != nil {
		ro = a.getFirstResourceOwner(state, client, *data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(state, true, req.Scope, data.RedirectURI, client, ro)

	// delete authorization code
	a.deleteToken(state, a.policy.Token, code.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) revocationEndpoint(state *state) {
	// begin trace
	state.tracer.Push("flame/Authenticator.revocationEndpoint")

	// parse authorization request
	req, err := revocation.ParseRequest(state.request)
	stack.AbortIf(err)

	// get client
	client := a.findFirstClient(state, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	claims, _, err := a.policy.ParseJWT(req.Token)
	if err != nil {
		state.tracer.Pop()
		return
	}

	// parse id
	id, err := coal.FromHex(claims.Id)
	if err != nil {
		state.tracer.Pop()
		return
	}

	// delete token
	a.deleteToken(state, a.policy.Token, id)

	// write header
	state.writer.WriteHeader(http.StatusOK)

	// finish trace
	state.tracer.Pop()
}

func (a *Authenticator) issueTokens(state *state, refreshable bool, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
	// begin trace
	state.tracer.Push("flame/Authenticator.issueTokens")

	// prepare expiration
	atExpiry := time.Now().Add(a.policy.AccessTokenLifespan)
	rtExpiry := time.Now().Add(a.policy.RefreshTokenLifespan)

	// save access token
	at := a.saveToken(state, a.policy.Token, AccessToken, scope, atExpiry, redirectURI, client, resourceOwner)

	// generate new access token
	atSignature, err := a.policy.GenerateJWT(at, client, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := bearer.NewTokenResponse(atSignature, int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	// issue a refresh token if requested
	if refreshable {
		// save refresh token
		rt := a.saveToken(state, a.policy.Token, RefreshToken, scope, rtExpiry, redirectURI, client, resourceOwner)

		// generate new refresh token
		rtSignature, err := a.policy.GenerateJWT(rt, client, resourceOwner)
		stack.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	// finish trace
	state.tracer.Pop()

	return res
}

func (a *Authenticator) issueCode(state *state, scope oauth2.Scope, redirectURI string, client Client, resourceOwner ResourceOwner) *oauth2.CodeResponse {
	// begin trace
	state.tracer.Push("flame/Authenticator.issueCode")

	// prepare expiration
	expiry := time.Now().Add(a.policy.AuthorizationCodeLifespan)

	// save authorization code
	code := a.saveToken(state, a.policy.Token, AuthorizationCode, scope, expiry, redirectURI, client, resourceOwner)

	// generate new access token
	signature, err := a.policy.GenerateJWT(code, client, resourceOwner)
	stack.AbortIf(err)

	// prepare response
	res := oauth2.NewCodeResponse(signature, "", "")

	// finish trace
	state.tracer.Pop()

	return res
}

func (a *Authenticator) findFirstClient(state *state, id string) Client {
	// begin trace
	state.tracer.Push("flame/Authenticator.findFirstClient")

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.findClient(state, model, id)
		if c != nil {
			state.tracer.Pop()
			return c
		}
	}

	// finish trace
	state.tracer.Pop()

	return nil
}

func (a *Authenticator) findClient(state *state, model Client, id string) Client {
	// begin trace
	state.tracer.Push("flame/Authenticator.findClient")

	// prepare object
	obj := model.Meta().Make()

	// lookup id field
	field := coal.F(model, coal.L(model, "flame-client-id", true))

	// prepare filter
	filters := []bson.M{
		{field: id},
	}

	// add additional filter if provided
	if a.policy.ClientFilter != nil {
		// run filter function
		filter, err := a.policy.ClientFilter(model, state.request)
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

	// query db
	err := state.store.TC(state.tracer, model).FindOne(nil, query).Decode(obj)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize model
	client := coal.Init(obj).(Client)

	// finish trace
	state.tracer.Pop()

	return client
}

func (a *Authenticator) getFirstClient(state *state, id coal.ID) Client {
	// begin trace
	state.tracer.Push("flame/Authenticator.getFirstClient")

	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.getClient(state, model, id)
		if c != nil {
			state.tracer.Pop()
			return c
		}
	}

	// finish trace
	state.tracer.Pop()

	return nil
}

func (a *Authenticator) getClient(state *state, model Client, id coal.ID) Client {
	// begin trace
	state.tracer.Push("flame/Authenticator.getClient")

	// prepare object
	obj := model.Meta().Make()

	// query db
	err := state.store.TC(state.tracer, model).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(obj)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize model
	client := coal.Init(obj).(Client)

	// finish trace
	state.tracer.Pop()

	return client
}

func (a *Authenticator) findFirstResourceOwner(state *state, client Client, id string) ResourceOwner {
	// begin trace
	state.tracer.Push("flame/Authenticator.findFirstResourceOwner")

	// check all available models in order
	for _, model := range a.policy.ResourceOwners(client) {
		ro := a.findResourceOwner(state, model, id)
		if ro != nil {
			state.tracer.Pop()
			return ro
		}
	}

	// finish trace
	state.tracer.Pop()

	return nil
}

func (a *Authenticator) findResourceOwner(state *state, model ResourceOwner, id string) ResourceOwner {
	// begin trace
	state.tracer.Push("flame/Authenticator.findResourceOwner")

	// prepare object
	obj := coal.Init(model).Meta().Make()

	// lookup id field
	field := coal.F(model, coal.L(model, "flame-resource-owner-id", true))

	// prepare filter
	filters := []bson.M{
		{field: id},
	}

	// add additional filter if provided
	if a.policy.ResourceOwnerFilter != nil {
		// run filter function
		filter, err := a.policy.ResourceOwnerFilter(model, state.request)
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

	// query db
	err := state.store.TC(state.tracer, model).FindOne(nil, query).Decode(obj)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize model
	resourceOwner := coal.Init(obj).(ResourceOwner)

	// finish trace
	state.tracer.Pop()

	return resourceOwner
}

func (a *Authenticator) getFirstResourceOwner(state *state, client Client, id coal.ID) ResourceOwner {
	// begin trace
	state.tracer.Push("flame/Authenticator.getFirstResourceOwner")

	// check all available models in order
	for _, model := range a.policy.ResourceOwners(client) {
		ro := a.getResourceOwner(state, model, id)
		if ro != nil {
			state.tracer.Pop()
			return ro
		}
	}

	// finish trace
	state.tracer.Pop()

	return nil
}

func (a *Authenticator) getResourceOwner(state *state, model ResourceOwner, id coal.ID) ResourceOwner {
	// begin trace
	state.tracer.Push("flame/Authenticator.getResourceOwner")

	// prepare object
	obj := coal.Init(model).Meta().Make()

	// query db
	err := state.store.TC(state.tracer, model).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(obj)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize model
	resourceOwner := coal.Init(obj).(ResourceOwner)

	// finish trace
	state.tracer.Pop()

	return resourceOwner
}

func (a *Authenticator) getToken(state *state, model GenericToken, id coal.ID) GenericToken {
	// begin trace
	state.tracer.Push("flame/Authenticator.getToken")

	// prepare object
	obj := model.Meta().Make()

	// fetch access token
	err := state.store.TC(state.tracer, model).FindOne(nil, bson.M{
		"_id": id,
	}).Decode(obj)
	if err == mongo.ErrNoDocuments {
		return nil
	}
	stack.AbortIf(err)

	// initialize access token
	accessToken := coal.Init(obj).(GenericToken)

	// finish trace
	state.tracer.Pop()

	return accessToken
}

func (a *Authenticator) saveToken(state *state, model GenericToken, typ TokenType, scope []string, expiresAt time.Time, redirectURI string, client Client, resourceOwner ResourceOwner) GenericToken {
	// begin trace
	state.tracer.Push("flame/Authenticator.saveToken")

	// prepare access token
	token := model.Meta().Make().(GenericToken)

	// get resource owner id
	var roID *coal.ID
	if resourceOwner != nil {
		roID = coal.P(resourceOwner.ID())
	}

	// set access token data
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

	// save access token
	_, err := state.store.TC(state.tracer, token).InsertOne(nil, token)
	stack.AbortIf(err)

	// finish trace
	state.tracer.Pop()

	return token
}

func (a *Authenticator) deleteToken(state *state, model GenericToken, id coal.ID) {
	// begin trace
	state.tracer.Push("flame/Authenticator.deleteToken")

	// delete token
	_, err := state.store.TC(state.tracer, model).DeleteOne(nil, bson.M{
		"_id": id,
	})
	if err == mongo.ErrNoDocuments {
		err = nil
	}
	stack.AbortIf(err)

	// finish trace
	state.tracer.Pop()
}
