// Package flame implements an authentication manager that provides OAuth2
// compatible authentication with JWT tokens.
package flame

import (
	"context"
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

type ctxKey int

// AccessTokenContextKey is the key used to save the access token in a context.
const AccessTokenContextKey ctxKey = iota

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
	// check secret
	if len(policy.Secret) < 16 {
		panic("flame: secret must be longer than 16 characters")
	}

	// initialize models
	coal.Init(policy.AccessToken)
	coal.Init(policy.RefreshToken)

	// initialize clients
	for _, model := range policy.Clients {
		coal.Init(model)
	}

	// initialize resource owners
	for _, model := range policy.ResourceOwners {
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
			// directly write potential oauth2 errors
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

		// try to call the controllers general handler
		if len(s) > 0 {
			if s[0] == "authorize" {
				a.authorizationEndpoint(w, r)
				return
			} else if s[0] == "token" {
				a.tokenEndpoint(w, r)
				return
			} else if s[0] == "revoke" {
				a.revocationEndpoint(w, r)
				return
			}
		}

		// write not found error
		w.WriteHeader(http.StatusNotFound)
	})
}

// Authorizer returns a middleware that can be used to authorize a request by
// requiring an access token with the provided scope to be granted.
func (a *Authenticator) Authorizer(scope string, force bool) func(http.Handler) http.Handler {
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
				// directly write potential bearer errors
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

			// get token
			accessToken := a.getAccessToken(bson.ObjectIdHex(claims.Id))
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

			// call next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Authenticator) authorizationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(r)
	stack.AbortIf(err)

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		stack.Abort(oauth2.InvalidRequest("unknown response type"))
	}

	// get client
	client := a.getFirstClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate redirect uri
	if !client.ValidRedirectURI(req.RedirectURI) {
		stack.Abort(oauth2.InvalidRequest("invalid redirect uri"))
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		if a.policy.ImplicitGrant {
			a.handleImplicitGrant(w, r, req, client)
			return
		}
	}

	// response type is unsupported
	stack.Abort(oauth2.UnsupportedResponseType(""))
}

func (a *Authenticator) handleImplicitGrant(w http.ResponseWriter, r *http.Request, req *oauth2.AuthorizationRequest, client Client) {
	// check request method
	if r.Method == "GET" {
		stack.Abort(oauth2.InvalidRequest("unallowed request method").SetRedirect(req.RedirectURI, req.State, true))
	}

	// get credentials
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// get resource owner
	resourceOwner := a.findFirstResourceOwner(username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope("").SetRedirect(req.RedirectURI, req.State, true))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(false, scope, client, resourceOwner)

	// redirect response
	res.SetRedirect(req.RedirectURI, req.State, true)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) tokenEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse token request
	req, err := oauth2.ParseTokenRequest(r)
	stack.AbortIf(err)

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		stack.Abort(oauth2.InvalidRequest("unknown grant type"))
	}

	// get client
	client := a.getFirstClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		if a.policy.PasswordGrant {
			a.handleResourceOwnerPasswordCredentialsGrant(w, req, client)
			return
		}
	case oauth2.ClientCredentialsGrantType:
		if a.policy.ClientCredentialsGrant {
			a.handleClientCredentialsGrant(w, req, client)
			return
		}
	case oauth2.RefreshTokenGrantType:
		a.handleRefreshTokenGrant(w, req, client)
		return
	}

	// grant type is unsupported
	stack.Abort(oauth2.UnsupportedGrantType(""))
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// get resource owner
	resourceOwner := a.findFirstResourceOwner(req.Username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied(""))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(true, scope, client, resourceOwner)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) handleClientCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// validate & grant scope
	scope, err := a.policy.GrantStrategy(&GrantRequest{
		Scope:  req.Scope,
		Client: client,
	})
	if err == ErrGrantRejected {
		stack.Abort(oauth2.AccessDenied(""))
	} else if err == ErrInvalidScope {
		stack.Abort(oauth2.InvalidScope(""))
	} else if err != nil {
		stack.Abort(err)
	}

	// issue access token
	res := a.issueTokens(true, scope, client, nil)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) handleRefreshTokenGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// parse token
	claims, expired, err := a.policy.ParseToken(req.RefreshToken)
	if expired {
		stack.Abort(oauth2.InvalidGrant("expired refresh token"))
	} else if err != nil {
		stack.Abort(oauth2.InvalidRequest("malformed token"))
	}

	// get stored refresh token by signature
	rt := a.getRefreshToken(bson.ObjectIdHex(claims.Id))
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
		ro = a.getFirstResourceOwner(*data.ResourceOwnerID)
	}

	// issue tokens
	res := a.issueTokens(true, req.Scope, client, ro)

	// delete refresh token
	a.deleteRefreshToken(rt.ID(), client.ID())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) revocationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := revocation.ParseRequest(r)
	stack.AbortIf(err)

	// get client
	client := a.getFirstClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("unknown client"))
	}

	// parse token
	claims, _, err := a.policy.ParseToken(req.Token)
	if err != nil {
		return
	}

	// delete access token
	a.deleteAccessToken(bson.ObjectIdHex(claims.Id), client.ID())

	// delete refresh token
	a.deleteRefreshToken(bson.ObjectIdHex(claims.Id), client.ID())

	// write header
	w.WriteHeader(http.StatusOK)
}

func (a *Authenticator) issueTokens(refreshable bool, scope oauth2.Scope, client Client, resourceOwner ResourceOwner) *oauth2.TokenResponse {
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
		roID := resourceOwner.ID()
		accessTokenData.ResourceOwnerID = &roID
	}

	// save access token
	at := a.saveAccessToken(accessTokenData)

	// generate new access token
	atSignature, err := a.policy.GenerateToken(at.ID(), time.Now(), atExpiry, client, resourceOwner)
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
			roID := resourceOwner.ID()
			refreshTokenData.ResourceOwnerID = &roID
		}

		// save refresh token
		rt := a.saveRefreshToken(refreshTokenData)

		// generate new refresh token
		rtSignature, err := a.policy.GenerateToken(rt.ID(), time.Now(), rtExpiry, client, resourceOwner)
		stack.AbortIf(err)

		// set refresh token
		res.RefreshToken = rtSignature
	}

	// run automated cleanup if enabled
	if a.policy.AutomatedCleanup {
		a.cleanup()
	}

	return res
}

func (a *Authenticator) getFirstClient(id string) Client {
	// check all available models in order
	for _, model := range a.policy.Clients {
		c := a.getClient(model, id)
		if c != nil {
			return c
		}
	}

	return nil
}

func (a *Authenticator) getClient(model Client, id string) Client {
	// prepare object
	obj := model.Meta().Make()

	// get store
	store := a.store.Copy()
	defer store.Close()

	// get description
	desc := model.DescribeClient()

	// get id field
	field := model.Meta().FindField(desc.IdentifierField)

	// query db
	err := store.C(model).Find(bson.M{
		field.BSONName: id,
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

func (a *Authenticator) findFirstResourceOwner(id string) ResourceOwner {
	// check all available models in order
	for _, model := range a.policy.ResourceOwners {
		ro := a.findResourceOwner(model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) findResourceOwner(model ResourceOwner, id string) ResourceOwner {
	// prepare object
	obj := model.Meta().Make()

	// get store
	store := a.store.Copy()
	defer store.Close()

	// get description
	desc := model.DescribeResourceOwner()

	// get id field
	field := model.Meta().FindField(desc.IdentifierField)

	// query db
	err := store.C(model).Find(bson.M{
		field.BSONName: id,
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

func (a *Authenticator) getFirstResourceOwner(id bson.ObjectId) ResourceOwner {
	// check all available models in order
	for _, model := range a.policy.ResourceOwners {
		ro := a.getResourceOwner(model, id)
		if ro != nil {
			return ro
		}
	}

	return nil
}

func (a *Authenticator) getResourceOwner(model ResourceOwner, id bson.ObjectId) ResourceOwner {
	// prepare object
	obj := model.Meta().Make()

	// get store
	store := a.store.Copy()
	defer store.Close()

	// query db
	err := store.C(model).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	resourceOwner := coal.Init(obj).(ResourceOwner)

	return resourceOwner
}

func (a *Authenticator) getAccessToken(id bson.ObjectId) Token {
	return a.getToken(a.policy.AccessToken, id)
}

func (a *Authenticator) getRefreshToken(id bson.ObjectId) Token {
	return a.getToken(a.policy.RefreshToken, id)
}

func (a *Authenticator) getToken(t Token, id bson.ObjectId) Token {
	// prepare object
	obj := t.Meta().Make()

	// get store
	store := a.store.Copy()
	defer store.Close()

	// fetch access token
	err := store.C(t).FindId(id).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize access token
	accessToken := coal.Init(obj).(Token)

	return accessToken
}

func (a *Authenticator) saveAccessToken(d *TokenData) Token {
	return a.saveToken(a.policy.AccessToken, d)
}

func (a *Authenticator) saveRefreshToken(d *TokenData) Token {
	return a.saveToken(a.policy.RefreshToken, d)
}

func (a *Authenticator) saveToken(t Token, d *TokenData) Token {
	// prepare access token
	token := t.Meta().Make().(Token)

	// set access token data
	token.SetTokenData(d)

	// get store
	store := a.store.Copy()
	defer store.Close()

	// save access token
	err := store.C(token).Insert(token)

	// abort on error
	stack.AbortIf(err)

	return token
}

func (a *Authenticator) deleteAccessToken(id bson.ObjectId, clientID bson.ObjectId) {
	a.deleteToken(a.policy.AccessToken, id, clientID)
}

func (a *Authenticator) deleteRefreshToken(id bson.ObjectId, clientID bson.ObjectId) {
	a.deleteToken(a.policy.RefreshToken, id, clientID)
}

func (a *Authenticator) deleteToken(t Token, id bson.ObjectId, clientID bson.ObjectId) {
	// get store
	store := a.store.Copy()
	defer store.Close()

	// delete token
	err := store.C(t).RemoveId(id)
	if err == mgo.ErrNotFound {
		err = nil
	}

	// abort on critical error
	stack.AbortIf(err)
}

func (a *Authenticator) cleanup() {
	// remove all expired access tokens
	a.cleanupToken(a.policy.AccessToken)

	// remove all expired refresh tokens
	a.cleanupToken(a.policy.RefreshToken)
}

func (a *Authenticator) cleanupToken(t Token) {
	// get store
	store := a.store.Copy()
	defer store.Close()

	// get description
	desc := t.DescribeToken()

	// get expires at field
	field := t.Meta().FindField(desc.ExpiresAtField)

	// remove all records
	_, err := store.C(t).RemoveAll(bson.M{
		field.BSONName: bson.M{
			"$lt": time.Now(),
		},
	})

	// abort on error
	stack.AbortIf(err)
}
