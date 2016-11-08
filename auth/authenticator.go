// Package auth implements an authenticator component that provides OAuth2
// compatible authentication.
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/256dpi/stack"
	"github.com/gonfire/fire"
	"github.com/gonfire/oauth2"
	"github.com/gonfire/oauth2/bearer"
	"github.com/gonfire/oauth2/hmacsha"
	"github.com/gonfire/oauth2/revocation"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type ctxKex int

// AccessTokenContextKey is the key used to save the access token in a context.
const AccessTokenContextKey ctxKex = iota

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant.
type Authenticator struct {
	store  *fire.Store
	policy *Policy

	Reporter func(error)
}

// New constructs a new Authenticator from a store and policy.
func New(store *fire.Store, policy *Policy) *Authenticator {
	// check secret
	if len(policy.Secret) < 16 {
		panic("Secret must be longer than 16 characters")
	}

	// initialize models
	fire.Init(policy.AccessToken)
	fire.Init(policy.RefreshToken)
	fire.Init(policy.Client)
	fire.Init(policy.ResourceOwner)

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
// requiring an access token with the provided scopes to be granted.
func (a *Authenticator) Authorizer(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			token, err := hmacsha.Parse(a.policy.Secret, tk)
			if err != nil {
				stack.Abort(bearer.InvalidToken("Malformed token"))
			}

			// get token
			accessToken := a.getAccessToken(token.SignatureString())
			if accessToken == nil {
				stack.Abort(bearer.InvalidToken("Unkown token"))
			}

			// get additional data
			data := accessToken.GetTokenData()

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				stack.Abort(bearer.InvalidToken("Expired token"))
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
		stack.Abort(oauth2.InvalidRequest("Unknown response type"))
	}

	// get client
	client := a.getClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// validate redirect uri
	if !client.ValidRedirectURI(req.RedirectURI) {
		stack.Abort(oauth2.InvalidRequest("Invalid redirect URI"))
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
		stack.Abort(oauth2.InvalidRequest("Unallowed request method").SetRedirect(req.RedirectURI, req.State, true))
	}

	// get credentials
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// get resource owner
	resourceOwner := a.getResourceOwner(username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		stack.Abort(oauth2.AccessDenied("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// validate & grant scope
	granted, scope := a.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		stack.Abort(oauth2.InvalidScope("").SetRedirect(req.RedirectURI, req.State, true))
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res := a.issueTokens(false, scope, client.ID(), &rid)

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
		stack.Abort(oauth2.InvalidRequest("Unknown grant type"))
	}

	// get client
	client := a.getClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
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
	resourceOwner := a.getResourceOwner(req.Username)
	if resourceOwner == nil {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		stack.Abort(oauth2.AccessDenied(""))
	}

	// validate & grant scope
	granted, scope := a.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		stack.Abort(oauth2.InvalidScope(""))
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res := a.issueTokens(true, scope, client.ID(), &rid)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) handleClientCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// validate & grant scope
	granted, scope := a.policy.GrantStrategy(&GrantRequest{
		Scope:  req.Scope,
		Client: client,
	})
	if !granted {
		stack.Abort(oauth2.InvalidScope(""))
	}

	// issue access token
	res := a.issueTokens(true, scope, client.ID(), nil)

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) handleRefreshTokenGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// parse refresh token
	refreshToken, err := hmacsha.Parse(a.policy.Secret, req.RefreshToken)
	if err != nil {
		stack.Abort(oauth2.InvalidRequest(err.Error()))
	}

	// get stored refresh token by signature
	rt := a.getRefreshToken(refreshToken.SignatureString())
	if rt == nil {
		stack.Abort(oauth2.InvalidGrant("Unknown refresh token"))
	}

	// get data
	data := rt.GetTokenData()

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		stack.Abort(oauth2.InvalidGrant("Expired refresh token"))
	}

	// validate ownership
	if data.ClientID != client.ID() {
		stack.Abort(oauth2.InvalidGrant("Invalid refresh token ownership"))
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !oauth2.Scope(data.Scope).Includes(req.Scope) {
		stack.Abort(oauth2.InvalidScope("Scope exceeds the originally granted scope"))
	}

	// issue tokens
	res := a.issueTokens(true, req.Scope, client.ID(), data.ResourceOwnerID)

	// delete refresh token
	a.deleteRefreshToken(refreshToken.SignatureString())

	// write response
	stack.AbortIf(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) revocationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := revocation.ParseRequest(r)
	stack.AbortIf(err)

	// get client
	client := a.getClient(req.ClientID)
	if client == nil {
		stack.Abort(oauth2.InvalidClient("Unknown client"))
	}

	// parse token
	token, err := hmacsha.Parse(a.policy.Secret, req.Token)
	if err != nil {
		// we do not care about wrong tokens TODO: Sure?
		return
	}

	// TODO: Only revoke tokens that belong to the provided client.

	// delete access token
	a.deleteAccessToken(token.SignatureString())

	// delete refresh token
	a.deleteRefreshToken(token.SignatureString())

	// write header
	w.WriteHeader(http.StatusOK)
}

func (a *Authenticator) issueTokens(refreshable bool, s oauth2.Scope, cID bson.ObjectId, roID *bson.ObjectId) *oauth2.TokenResponse {
	// generate new access token
	accessToken, err := hmacsha.Generate(a.policy.Secret, 32)
	stack.AbortIf(err)

	// generate new refresh token
	refreshToken, err := hmacsha.Generate(a.policy.Secret, 32)
	stack.AbortIf(err)

	// prepare response
	res := bearer.NewTokenResponse(accessToken.String(), int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = s

	// set refresh token if requested
	if refreshable {
		res.RefreshToken = refreshToken.String()
	}

	// create access token data
	accessTokenData := &TokenData{
		Signature:       accessToken.SignatureString(),
		Scope:           s,
		ExpiresAt:       time.Now().Add(a.policy.AccessTokenLifespan),
		ClientID:        cID,
		ResourceOwnerID: roID,
	}

	// save access token
	a.saveAccessToken(accessTokenData)

	if refreshable {
		// create refresh token data
		refreshTokenData := &TokenData{
			Signature:       refreshToken.SignatureString(),
			Scope:           s,
			ExpiresAt:       time.Now().Add(a.policy.RefreshTokenLifespan),
			ClientID:        cID,
			ResourceOwnerID: roID,
		}

		// save refresh token
		a.saveRefreshToken(refreshTokenData)
	}

	// run automated cleanup if enabled
	if a.policy.AutomatedCleanup {
		a.cleanup()
	}

	return res
}

func (a *Authenticator) getClient(id string) Client {
	// prepare object
	obj := a.policy.Client.Meta().Make()

	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get id field
	field := a.policy.Client.Meta().FindField(a.policy.Client.DescribeClient())

	// query db
	err := store.C(a.policy.Client).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	client := fire.Init(obj).(Client)

	return client
}

func (a *Authenticator) getResourceOwner(id string) ResourceOwner {
	// prepare object
	obj := a.policy.ResourceOwner.Meta().Make()

	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get id field
	field := a.policy.ResourceOwner.Meta().FindField(a.policy.ResourceOwner.DescribeResourceOwner())

	// query db
	err := store.C(a.policy.ResourceOwner).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize model
	resourceOwner := fire.Init(obj).(ResourceOwner)

	return resourceOwner
}

func (a *Authenticator) getAccessToken(signature string) Token {
	return a.getToken(a.policy.AccessToken, signature)
}

func (a *Authenticator) getRefreshToken(signature string) Token {
	return a.getToken(a.policy.RefreshToken, signature)
}

func (a *Authenticator) getToken(t Token, signature string) Token {
	// prepare object
	obj := t.Meta().Make()

	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get token id field name
	fieldName, _ := t.DescribeToken()

	// get signature field
	field := t.Meta().FindField(fieldName)

	// fetch access token
	err := store.C(t).Find(bson.M{
		field.BSONName: signature,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil
	}

	// abort on error
	stack.AbortIf(err)

	// initialize access token
	accessToken := fire.Init(obj).(Token)

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

	// ensure store gets closed
	defer store.Close()

	// save access token
	err := store.C(token).Insert(token)

	// abort on error
	stack.AbortIf(err)

	return token
}

func (a *Authenticator) deleteAccessToken(signature string) {
	a.deleteToken(a.policy.AccessToken, signature)
}

func (a *Authenticator) deleteRefreshToken(signature string) {
	a.deleteToken(a.policy.RefreshToken, signature)
}

func (a *Authenticator) deleteToken(t Token, signature string) {
	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get token id field name
	fieldName, _ := t.DescribeToken()

	// get signature field
	field := t.Meta().FindField(fieldName)

	// fetch access token
	_, err := store.C(t).RemoveAll(bson.M{
		field.BSONName: signature,
	})

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

	// ensure store gets closed
	defer store.Close()

	// get access token expires at field name
	_, fieldName := t.DescribeToken()

	// get expires at field
	field := t.Meta().FindField(fieldName)

	// remove all records
	_, err := store.C(t).RemoveAll(bson.M{
		field.BSONName: bson.M{
			"$lt": time.Now(),
		},
	})

	// abort on error
	stack.AbortIf(err)
}
