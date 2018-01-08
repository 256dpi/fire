// Package flame implements an authenticator that provides OAuth2 compatible
// authentication with JWT tokens.
package flame

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/256dpi/oauth2"
	"github.com/256dpi/oauth2/bearer"
	"github.com/256dpi/oauth2/revocation"
	"github.com/256dpi/stack"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
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

// TODO: Implement tracing.

type state struct {
	request *http.Request
	writer  http.ResponseWriter
	store   *coal.SubStore
}

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant.
type Authenticator struct {
	store  *coal.Store
	policy *Policy

	Reporter func(error)
}

// NewAuthenticator constructs a new Authenticator from a store and policy.
func NewAuthenticator(store *coal.Store, policy *Policy) *Authenticator {
	// initialize models
	coal.Init(policy.AccessToken)
	coal.Init(policy.RefreshToken)

	// initialize clients
	for _, model := range policy.Clients {
		coal.Init(model)
	}

	return &Authenticator{
		store:  store,
		policy: policy,
	}
}

// Endpoint returns a handler for the common token and authorize endpoint.
func (a *Authenticator) Endpoint(prefix string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// continue any previous aborts
		defer stack.Resume(func(err error) {
			// directly write oauth2 errors
			if oauth2Error, ok := err.(*oauth2.Error); ok {
				oauth2.WriteError(w, oauth2Error)
				return
			}

			// otherwise report critical errors
			if a.Reporter != nil {
				a.Reporter(err)
			}

			// ignore errors caused by writing critical errors
			oauth2.WriteError(w, oauth2.ServerError(""))
		})

		// trim and split path
		s := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")

		// check length
		if len(s) != 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// check resource
		if s[0] != "authorize" && s[0] != "token" && s[0] != "revoke" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// copy store
		store := a.store.Copy()
		defer store.Close()

		// create
		state := &state{
			request: r,
			writer:  w,
			store:   store,
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
			// immediately pass on request if force is not set and there is
			// no authentication information provided
			if !force && r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
			}

			// continue any previous aborts
			defer stack.Resume(func(err error) {
				// directly write bearer errors
				if bearerError, ok := err.(*bearer.Error); ok {
					bearer.WriteError(w, bearerError)
					return
				}

				// otherwise report critical errors
				if a.Reporter != nil {
					a.Reporter(err)
				}

				// ignore errors caused by writing critical errors
				bearer.WriteError(w, bearer.ServerError())
			})

			// parse scope
			s := oauth2.ParseScope(scope)

			// parse bearer token
			tk, err := bearer.ParseToken(r)
			stack.AbortIf(err)

			// parse token
			claims, expired, err := a.policy.ParseToken(tk)
			if expired {
				stack.Abort(bearer.InvalidToken("expired token"))
			} else if err != nil {
				stack.Abort(bearer.InvalidToken("malformed token"))
			}

			// copy tore
			store := a.store.Copy()
			defer store.Close()

			// create state
			state := &state{
				request: r,
				writer:  w,
				store:   store,
			}

			// get token
			accessToken := a.getToken(state, a.policy.AccessToken, bson.ObjectIdHex(claims.Id))
			if accessToken == nil {
				stack.Abort(bearer.InvalidToken("unknown token"))
			}

			// get additional data
			data := accessToken.GetTokenData()

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
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// get resource owner
			resourceOwner := a.getFirstResourceOwner(state, client, *data.ResourceOwnerID)
			if client == nil {
				stack.Abort(errors.New("missing resource owner"))
			}

			// create new context with resource owner
			ctx = context.WithValue(ctx, ResourceOwnerContextKey, resourceOwner)

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Authenticator) authorizationEndpoint(state *state) {
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
		if a.policy.ImplicitGrant {
			a.handleImplicitGrant(state, req, client)
			return
		}
	}

	// response type is unsupported
	stack.Abort(oauth2.UnsupportedResponseType(""))
}

func (a *Authenticator) handleImplicitGrant(state *state, req *oauth2.AuthorizationRequest, client Client) {
	// check request method
	if state.request.Method == "GET" {
		stack.Abort(oauth2.InvalidRequest("unallowed request method").SetRedirect(req.RedirectURI, req.State, true))
	}

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
	res := a.issueTokens(state, false, scope, client, resourceOwner)

	// redirect response
	res.SetRedirect(req.RedirectURI, req.State, true)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))
}

func (a *Authenticator) tokenEndpoint(state *state) {
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
		if a.policy.PasswordGrant {
			a.handleResourceOwnerPasswordCredentialsGrant(state, req, client)
			return
		}
	case oauth2.ClientCredentialsGrantType:
		if a.policy.ClientCredentialsGrant {
			a.handleClientCredentialsGrant(state, req, client)
			return
		}
	case oauth2.RefreshTokenGrantType:
		a.handleRefreshTokenGrant(state, req, client)
		return
	}

	// grant type is unsupported
	stack.Abort(oauth2.UnsupportedGrantType(""))
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(state *state, req *oauth2.TokenRequest, client Client) {
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
	res := a.issueTokens(state, true, scope, client, resourceOwner)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))
}

func (a *Authenticator) handleClientCredentialsGrant(state *state, req *oauth2.TokenRequest, client Client) {
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
	res := a.issueTokens(state, true, scope, client, nil)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))
}

func (a *Authenticator) handleRefreshTokenGrant(state *state, req *oauth2.TokenRequest, client Client) {
	// parse token
	claims, expired, err := a.policy.ParseToken(req.RefreshToken)
	if expired {
		stack.Abort(oauth2.InvalidGrant("expired refresh token"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// get stored refresh token by signature
	rt := a.getToken(state, a.policy.RefreshToken, bson.ObjectIdHex(claims.Id))
	if rt == nil {
		stack.Abort(oauth2.InvalidGrant("unknown refresh token"))
	}

	// get data
	data := rt.GetTokenData()

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
	res := a.issueTokens(state, true, req.Scope, client, ro)

	// delete refresh token
	a.deleteToken(state, a.policy.RefreshToken, rt.ID(), client.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(state.writer, res))
}

func (a *Authenticator) revocationEndpoint(state *state) {
	// parse authorization request
	req, err := revocation.ParseRequest(state.request)
	stack.AbortIf(err)

	// get client
	client := a.findFirstClient(state, req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	claims, _, err := a.policy.ParseToken(req.Token)
	if err != nil {
		return
	}

	// delete access token
	a.deleteToken(state, a.policy.AccessToken, bson.ObjectIdHex(claims.Id), client.ID())

	// delete refresh token
	a.deleteToken(state, a.policy.RefreshToken, bson.ObjectIdHex(claims.Id), client.ID())

	// write header
	state.writer.WriteHeader(http.StatusOK)
}

func (a *Authenticator) issueTokens(state *state, refreshable bool, scope oauth2.Scope, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
	// prepare expiration
	atExpiry := time.Now().Add(a.policy.AccessTokenLifespan)
	rtExpiry := time.Now().Add(a.policy.RefreshTokenLifespan)

	// create access token data
	accessTokenData := &TokenData{
		Scope:     scope,
		ExpiresAt: atExpiry,
		ClientID:  client.ID(),
	}

	// set resource owner id if available
	if resourceOwner != nil {
		accessTokenData.ResourceOwnerID = coal.P(resourceOwner.ID())
	}

	// save access token
	at := a.saveToken(state, a.policy.AccessToken, accessTokenData)

	// generate new access token
	atSignature, err := a.policy.GenerateToken(at.ID(), time.Now(), atExpiry, client, resourceOwner, at)
	stack.AbortIf(err)

	// prepare response
	res := bearer.NewTokenResponse(atSignature, int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	if refreshable {
		// create refresh token data
		refreshTokenData := &TokenData{
			Scope:     scope,
			ExpiresAt: rtExpiry,
			ClientID:  client.ID(),
		}

		// set resource owner id if available
		if resourceOwner != nil {
			refreshTokenData.ResourceOwnerID = coal.P(resourceOwner.ID())
		}

		// save refresh token
		rt := a.saveToken(state, a.policy.RefreshToken, refreshTokenData)

		// generate new refresh token
		rtSignature, err := a.policy.GenerateToken(rt.ID(), time.Now(), rtExpiry, client, resourceOwner, rt)
		stack.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	return res
}

func (a *Authenticator) findFirstClient(state *state, id string) Client {
	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.findClient(state, model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) findClient(state *state, model Client, id string) Client {
	// prepare object
	obj := model.Meta().Make()

	// get description
	desc := model.DescribeClient()

	// get id field
	field := coal.F(model, desc.IdentifierField)

	// query db
	err := state.store.C(model).Find(bson.M{
		field: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	client := coal.Init(obj).(Client)

	return client
}

func (a *Authenticator) getFirstClient(state *state, id bson.ObjectId) Client {
	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.getClient(state, model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) getClient(state *state, model Client, id bson.ObjectId) Client {
	// prepare object
	obj := model.Meta().Make()

	// query db
	err := state.store.C(model).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	client := coal.Init(obj).(Client)

	return client
}

func (a *Authenticator) findFirstResourceOwner(state *state, client Client, id string) ResourceOwner {
	// check all available models in order
	for _, model := range a.policy.ResourceOwners(client) {
		ro := a.findResourceOwner(state, model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) findResourceOwner(state *state, model ResourceOwner, id string) ResourceOwner {
	// prepare object
	obj := coal.Init(model).Meta().Make()

	// get description
	desc := model.DescribeResourceOwner()

	// get id field
	field := coal.F(model, desc.IdentifierField)

	// query db
	err := state.store.C(model).Find(bson.M{
		field: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	resourceOwner := coal.Init(obj).(ResourceOwner)

	return resourceOwner
}

func (a *Authenticator) getFirstResourceOwner(state *state, client Client, id bson.ObjectId) ResourceOwner {
	// check all available models in order
	for _, model := range a.policy.ResourceOwners(client) {
		ro := a.getResourceOwner(state, model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) getResourceOwner(state *state, model ResourceOwner, id bson.ObjectId) ResourceOwner {
	// prepare object
	obj := coal.Init(model).Meta().Make()

	// query db
	err := state.store.C(model).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	resourceOwner := coal.Init(obj).(ResourceOwner)

	return resourceOwner
}

func (a *Authenticator) getToken(state *state, t Token, id bson.ObjectId) Token {
	// prepare object
	obj := t.Meta().Make()

	// fetch access token
	err := state.store.C(t).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize access token
	accessToken := coal.Init(obj).(Token)

	return accessToken
}

func (a *Authenticator) saveToken(state *state, t Token, d *TokenData) Token {
	// prepare access token
	token := t.Meta().Make().(Token)

	// set access token data
	token.SetTokenData(d)

	// save access token
	err := state.store.C(token).Insert(token)

	// abort on error
	stack.AbortIf(err)

	return token
}

func (a *Authenticator) deleteToken(state *state, t Token, id bson.ObjectId, clientID bson.ObjectId) {
	// delete token
	err := state.store.C(t).RemoveId(id)
	if err == mgo.ErrNotFound {
		err = nil
	}

	// abort on critical error
	stack.AbortIf(err)
}
