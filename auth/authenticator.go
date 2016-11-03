// Package auth implements an authenticator component that provides OAuth2
// compatible authentication.
package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

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
			// parse scope
			s := oauth2.ParseScope(scope)

			// parse bearer token
			tk, res := bearer.ParseToken(r)
			if res != nil {
				a.assert(bearer.WriteError(w, res))
				return
			}

			// parse token
			token, err := hmacsha.Parse(a.policy.Secret, tk)
			if err != nil {
				a.assert(bearer.WriteError(w, bearer.InvalidToken("Malformed token")))
				return
			}

			// get token
			accessToken, err := a.getAccessToken(token.SignatureString())
			if err != nil {
				a.assert(bearer.WriteError(w, err))
				return
			} else if accessToken == nil {
				a.assert(bearer.WriteError(w, bearer.InvalidToken("Unkown token")))
				return
			}

			// get additional data
			data := accessToken.GetTokenData()

			// validate expiration
			if data.ExpiresAt.Before(time.Now()) {
				a.assert(bearer.WriteError(w, bearer.InvalidToken("Expired token")))
				return
			}

			// validate scope
			if !oauth2.Scope(data.Scope).Includes(s) {
				a.assert(bearer.WriteError(w, bearer.InsufficientScope(s.String())))
				return
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
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	}

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		a.assert(oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Unknown response type")))
		return
	}

	// get client
	client, err := a.getClient(req.ClientID)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	} else if client == nil {
		a.assert(oauth2.WriteError(w, oauth2.InvalidClient(req.State, "Unknown client")))
		return
	}

	// validate redirect uri
	if !client.ValidRedirectURI(req.RedirectURI) {
		a.assert(oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Invalid redirect URI")))
		return
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
	a.assert(oauth2.WriteError(w, oauth2.UnsupportedResponseType(req.State, oauth2.NoDescription)))
}

func (a *Authenticator) handleImplicitGrant(w http.ResponseWriter, r *http.Request, req *oauth2.AuthorizationRequest, client Client) {
	// check request method
	if r.Method == "GET" {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, oauth2.InvalidRequest(req.State, "Unallowed request method")))
		return
	}

	// get credentials
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// get resource owner
	resourceOwner, err := a.getResourceOwner(username)
	if err != nil {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, err))
		return
	} else if resourceOwner == nil {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, oauth2.AccessDenied(req.State, oauth2.NoDescription)))
		return
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, oauth2.AccessDenied(req.State, oauth2.NoDescription)))
		return
	}

	// validate & grant scope
	granted, scope := a.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, oauth2.InvalidScope(req.State, oauth2.NoDescription)))
		return
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res, err := a.issueTokens(false, scope, req.State, client.ID(), &rid)
	if err != nil {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, err))
		return
	}

	// write response
	a.assert(oauth2.RedirectTokenResponse(w, req.RedirectURI, res))
}

func (a *Authenticator) tokenEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse token request
	req, err := oauth2.ParseTokenRequest(r)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	}

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		a.assert(oauth2.WriteError(w, oauth2.InvalidRequest(oauth2.NoState, "Unknown grant type")))
		return
	}

	// get client
	client, err := a.getClient(req.ClientID)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	} else if client == nil {
		a.assert(oauth2.WriteError(w, oauth2.InvalidClient(oauth2.NoState, "Unknown client")))
		return
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
	a.assert(oauth2.WriteError(w, oauth2.UnsupportedGrantType(oauth2.NoState, oauth2.NoDescription)))
}

func (a *Authenticator) handleResourceOwnerPasswordCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// get resource owner
	resourceOwner, err := a.getResourceOwner(req.Username)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	} else if resourceOwner == nil {
		a.assert(oauth2.WriteError(w, oauth2.AccessDenied(oauth2.NoState, oauth2.NoDescription)))
		return
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		a.assert(oauth2.WriteError(w, oauth2.AccessDenied(oauth2.NoState, oauth2.NoDescription)))
		return
	}

	// validate & grant scope
	granted, scope := a.policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		a.assert(oauth2.WriteError(w, oauth2.InvalidScope(oauth2.NoState, oauth2.NoDescription)))
		return
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res, err := a.issueTokens(true, scope, oauth2.NoState, client.ID(), &rid)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	}

	// write response
	a.assert(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) handleClientCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		a.assert(oauth2.WriteError(w, oauth2.InvalidClient(oauth2.NoState, "Unknown client")))
		return
	}

	// validate & grant scope
	granted, scope := a.policy.GrantStrategy(&GrantRequest{
		Scope:  req.Scope,
		Client: client,
	})
	if !granted {
		a.assert(oauth2.WriteError(w, oauth2.InvalidScope(oauth2.NoState, oauth2.NoDescription)))
		return
	}

	// issue access token
	res, err := a.issueTokens(true, scope, oauth2.NoState, client.ID(), nil)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	}

	// write response
	a.assert(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) handleRefreshTokenGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// parse refresh token
	refreshToken, err := hmacsha.Parse(a.policy.Secret, req.RefreshToken)
	if err != nil {
		a.assert(oauth2.WriteError(w, oauth2.InvalidRequest(oauth2.NoState, err.Error())))
		return
	}

	// get stored refresh token by signature
	rt, err := a.getRefreshToken(refreshToken.SignatureString())
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	} else if rt == nil {
		a.assert(oauth2.WriteError(w, oauth2.InvalidGrant(oauth2.NoState, "Unknown refresh token")))
		return
	}

	// get data
	data := rt.GetTokenData()

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		a.assert(oauth2.WriteError(w, oauth2.InvalidGrant(oauth2.NoState, "Expired refresh token")))
		return
	}

	// validate ownership
	if data.ClientID != client.ID() {
		a.assert(oauth2.WriteError(w, oauth2.InvalidGrant(oauth2.NoState, "Invalid refresh token ownership")))
		return
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !oauth2.Scope(data.Scope).Includes(req.Scope) {
		a.assert(oauth2.WriteError(w, oauth2.InvalidScope(oauth2.NoState, "Scope exceeds the originally granted scope")))
		return
	}

	// issue tokens
	res, err := a.issueTokens(true, req.Scope, oauth2.NoState, client.ID(), data.ResourceOwnerID)
	if err != nil {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, err))
		return
	}

	// delete refresh token
	err = a.deleteRefreshToken(refreshToken.SignatureString())
	if err != nil {
		a.assert(oauth2.RedirectError(w, req.RedirectURI, true, err))
		return
	}

	// write response
	a.assert(oauth2.WriteTokenResponse(w, res))
}

func (a *Authenticator) revocationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := revocation.ParseRequest(r)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	}

	// get client
	client, err := a.getClient(req.ClientID)
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	} else if client == nil {
		a.assert(oauth2.WriteError(w, oauth2.InvalidClient(oauth2.NoState, "Unknown client")))
		return
	}

	// parse token
	token, err := hmacsha.Parse(a.policy.Secret, req.Token)
	if err != nil {
		// we do not care about wrong tokens
		return
	}

	// TODO: Only revoke tokens that belong to the provided client.

	// delete access token
	err = a.deleteAccessToken(token.SignatureString())
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	}

	// delete refresh token
	err = a.deleteRefreshToken(token.SignatureString())
	if err != nil {
		a.assert(oauth2.WriteError(w, err))
		return
	}

	// write header
	w.WriteHeader(http.StatusOK)
}

func (a *Authenticator) issueTokens(issueRefreshToken bool, scope oauth2.Scope, state string, clientID bson.ObjectId, resourceOwnerID *bson.ObjectId) (*oauth2.TokenResponse, error) {
	// generate new access token
	accessToken, err := hmacsha.Generate(a.policy.Secret, 32)
	if err != nil {
		a.assert(err)
		return nil, err
	}

	// generate new refresh token
	refreshToken, err := hmacsha.Generate(a.policy.Secret, 32)
	if err != nil {
		a.assert(err)
		return nil, err
	}

	// prepare response
	res := bearer.NewTokenResponse(accessToken.String(), int(a.policy.AccessTokenLifespan/time.Second))

	// set granted scope
	res.Scope = scope

	// set state
	res.State = state

	// set refresh token if requested
	if issueRefreshToken {
		res.RefreshToken = refreshToken.String()
	}

	// create access token data
	accessTokenData := &TokenData{
		Signature:       accessToken.SignatureString(),
		Scope:           scope,
		ExpiresAt:       time.Now().Add(a.policy.AccessTokenLifespan),
		ClientID:        clientID,
		ResourceOwnerID: resourceOwnerID,
	}

	// save access token
	_, err = a.saveAccessToken(accessTokenData)
	if err != nil {
		return nil, err
	}

	if issueRefreshToken {
		// create refresh token data
		refreshTokenData := &TokenData{
			Signature:       refreshToken.SignatureString(),
			Scope:           scope,
			ExpiresAt:       time.Now().Add(a.policy.RefreshTokenLifespan),
			ClientID:        clientID,
			ResourceOwnerID: resourceOwnerID,
		}

		// save refresh token
		_, err := a.saveRefreshToken(refreshTokenData)
		if err != nil {
			return nil, err
		}
	}

	// run automated cleanup if enabled
	if a.policy.AutomatedCleanup {
		err = a.cleanup()
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (a *Authenticator) getClient(id string) (Client, error) {
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
		return nil, nil
	} else if err != nil {
		a.assert(err)
		return nil, err
	}

	// initialize model
	client := fire.Init(obj).(Client)

	return client, nil
}

func (a *Authenticator) getResourceOwner(id string) (ResourceOwner, error) {
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
		return nil, nil
	} else if err != nil {
		a.assert(err)
		return nil, err
	}

	// initialize model
	resourceOwner := fire.Init(obj).(ResourceOwner)

	return resourceOwner, nil
}

func (a *Authenticator) getAccessToken(signature string) (Token, error) {
	return a.getToken(a.policy.AccessToken, signature)
}

func (a *Authenticator) getRefreshToken(signature string) (Token, error) {
	return a.getToken(a.policy.RefreshToken, signature)
}

func (a *Authenticator) getToken(tokenModel Token, signature string) (Token, error) {
	// prepare object
	obj := tokenModel.Meta().Make()

	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get token id field name
	fieldName, _ := tokenModel.DescribeToken()

	// get signature field
	field := tokenModel.Meta().FindField(fieldName)

	// fetch access token
	err := store.C(tokenModel).Find(bson.M{
		field.BSONName: signature,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		a.assert(err)
		return nil, err
	}

	// initialize access token
	accessToken := fire.Init(obj).(Token)

	return accessToken, nil
}

func (a *Authenticator) saveAccessToken(data *TokenData) (Token, error) {
	return a.saveToken(a.policy.AccessToken, data)
}

func (a *Authenticator) saveRefreshToken(data *TokenData) (Token, error) {
	return a.saveToken(a.policy.RefreshToken, data)
}

func (a *Authenticator) saveToken(tokenModel Token, data *TokenData) (Token, error) {
	// prepare access token
	token := tokenModel.Meta().Make().(Token)

	// set access token data
	token.SetTokenData(data)

	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// save access token
	err := store.C(token).Insert(token)
	if err != nil {
		a.assert(err)
		return nil, err
	}

	return token, nil
}

func (a *Authenticator) deleteAccessToken(signature string) error {
	return a.deleteToken(a.policy.AccessToken, signature)
}

func (a *Authenticator) deleteRefreshToken(signature string) error {
	return a.deleteToken(a.policy.RefreshToken, signature)
}

func (a *Authenticator) deleteToken(tokenModel Token, signature string) error {
	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get token id field name
	fieldName, _ := tokenModel.DescribeToken()

	// get signature field
	field := tokenModel.Meta().FindField(fieldName)

	// fetch access token
	err := store.C(tokenModel).Remove(bson.M{
		field.BSONName: signature,
	})
	if err == mgo.ErrNotFound {
		return nil
	} else if err != nil {
		a.assert(err)
		return err
	}

	return nil
}

func (a *Authenticator) cleanup() error {
	// remove all expired access tokens
	err := a.cleanupToken(a.policy.AccessToken)
	if err != nil {
		return err
	}

	// remove all expired refresh tokens
	err = a.cleanupToken(a.policy.RefreshToken)
	if err != nil {
		return err
	}

	return nil
}

func (a *Authenticator) cleanupToken(tokenModel Token) error {
	// get store
	store := a.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get access token expires at field name
	_, fieldName := tokenModel.DescribeToken()

	// get expires at field
	field := tokenModel.Meta().FindField(fieldName)

	// remove all records
	_, err := store.C(tokenModel).RemoveAll(bson.M{
		field.BSONName: bson.M{
			"$lt": time.Now(),
		},
	})
	if err != nil {
		a.assert(err)
		return err
	}

	return nil
}

func (a *Authenticator) assert(err error) {
	if err != nil && a.Reporter != nil {
		a.Reporter(err)
	}
}
