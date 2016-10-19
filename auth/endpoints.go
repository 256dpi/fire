package auth

import (
	"net/http"
	"time"

	"github.com/gonfire/oauth2"
	"github.com/gonfire/oauth2/bearer"
	"github.com/gonfire/oauth2/hmacsha"
	"gopkg.in/mgo.v2/bson"
)

func (a *Authenticator) AuthorizationEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse authorization request
	req, err := oauth2.ParseAuthorizationRequest(r)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	}

	// make sure the response type is known
	if !oauth2.KnownResponseType(req.ResponseType) {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Unknown response type"))
		return
	}

	// get client
	client, err := a.Storage.GetClient(req.ClientID)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if client == nil {
		oauth2.WriteError(w, oauth2.InvalidClient(req.State, "Unknown client"))
		return
	}

	// validate redirect uri
	if !client.ValidRedirectURI(req.RedirectURI) {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Invalid redirect URI"))
		return
	}

	// show info notice on a GET request
	if r.Method == "GET" {
		w.Write([]byte("This authentication server does not provide an authorization form."))
		return
	}

	// triage based on response type
	switch req.ResponseType {
	case oauth2.TokenResponseType:
		if a.Policy.ImplicitGrant {
			a.HandleImplicitGrant(w, r, req, client)
			return
		}
	}

	// response type is unsupported
	oauth2.WriteError(w, oauth2.UnsupportedResponseType(req.State, oauth2.NoDescription))
}

func (a *Authenticator) HandleImplicitGrant(w http.ResponseWriter, r *http.Request, req *oauth2.AuthorizationRequest, client Client) {
	// get credentials
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")

	// get resource owner
	resourceOwner, err := a.Storage.GetResourceOwner(username)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
		return
	} else if resourceOwner == nil {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.AccessDenied(req.State, oauth2.NoDescription))
		return
	}

	// validate password
	if !resourceOwner.ValidPassword(password) {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.AccessDenied(req.State, oauth2.NoDescription))
		return
	}

	// validate & grant scope
	granted, scope := a.Policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		oauth2.RedirectError(w, req.RedirectURI, true, oauth2.InvalidScope(req.State, oauth2.NoDescription))
		return
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res, err := a.issueTokens(false, scope, req.State, client.ID(), &rid)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.RedirectTokenResponse(w, req.RedirectURI, res)
}

func (a *Authenticator) TokenEndpoint(w http.ResponseWriter, r *http.Request) {
	// parse token request
	req, err := oauth2.ParseTokenRequest(r)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	}

	// make sure the grant type is known
	if !oauth2.KnownGrantType(req.GrantType) {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Unknown grant type"))
		return
	}

	// get client
	client, err := a.Storage.GetClient(req.ClientID)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if client == nil {
		oauth2.WriteError(w, oauth2.InvalidClient(req.State, "Unknown client"))
		return
	}

	// validate confidentiality
	/*if req.Confidential() && req.GrantType != oauth2.ClientCredentialsGrantType {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, "Confidential clients are only allowed for the client credentials grant."))
		return
	}*/

	// handle grant type
	switch req.GrantType {
	case oauth2.PasswordGrantType:
		if a.Policy.PasswordGrant {
			a.HandleResourceOwnerPasswordCredentialsGrant(w, req, client)
			return
		}
	case oauth2.ClientCredentialsGrantType:
		if a.Policy.ClientCredentialsGrant {
			a.HandleClientCredentialsGrant(w, req, client)
			return
		}
	case oauth2.RefreshTokenGrantType:
		a.HandleRefreshTokenGrant(w, req, client)
		return
	}

	// grant type is unsupported
	oauth2.WriteError(w, oauth2.UnsupportedGrantType(req.State, oauth2.NoDescription))
}

func (a *Authenticator) HandleResourceOwnerPasswordCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// get resource owner
	resourceOwner, err := a.Storage.GetResourceOwner(req.Username)
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if resourceOwner == nil {
		oauth2.WriteError(w, oauth2.AccessDenied(req.State, oauth2.NoDescription))
		return
	}

	// authenticate resource owner
	if !resourceOwner.ValidPassword(req.Password) {
		oauth2.WriteError(w, oauth2.AccessDenied(req.State, oauth2.NoDescription))
		return
	}

	// validate & grant scope
	granted, scope := a.Policy.GrantStrategy(&GrantRequest{
		Scope:         req.Scope,
		Client:        client,
		ResourceOwner: resourceOwner,
	})
	if !granted {
		oauth2.WriteError(w, oauth2.InvalidScope(req.State, oauth2.NoDescription))
		return
	}

	// get resource owner id
	rid := resourceOwner.ID()

	// issue access token
	res, err := a.issueTokens(true, scope, req.State, client.ID(), &rid)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.WriteTokenResponse(w, res)
}

func (a *Authenticator) HandleClientCredentialsGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// authenticate client
	if !client.ValidSecret(req.ClientSecret) {
		oauth2.WriteError(w, oauth2.InvalidClient(req.State, "Unknown client"))
		return
	}

	// validate & grant scope
	granted, scope := a.Policy.GrantStrategy(&GrantRequest{
		Scope:  req.Scope,
		Client: client,
	})
	if !granted {
		oauth2.WriteError(w, oauth2.InvalidScope(req.State, oauth2.NoDescription))
		return
	}

	// issue access token
	res, err := a.issueTokens(true, scope, req.State, client.ID(), nil)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.WriteTokenResponse(w, res)
}

func (a *Authenticator) HandleRefreshTokenGrant(w http.ResponseWriter, req *oauth2.TokenRequest, client Client) {
	// parse refresh token
	refreshToken, err := hmacsha.Parse(a.Policy.Secret, req.RefreshToken)
	if err != nil {
		oauth2.WriteError(w, oauth2.InvalidRequest(req.State, err.Error()))
		return
	}

	// get stored refresh token by signature
	rt, err := a.Storage.GetRefreshToken(refreshToken.SignatureString())
	if err != nil {
		oauth2.WriteError(w, err)
		return
	} else if rt == nil {
		oauth2.WriteError(w, oauth2.InvalidGrant(req.State, "Unknown refresh token"))
		return
	}

	// get data
	data := rt.GetTokenData()

	// validate expiration
	if data.ExpiresAt.Before(time.Now()) {
		oauth2.WriteError(w, oauth2.InvalidGrant(req.State, "Expired refresh token"))
		return
	}

	// validate ownership
	if data.ClientID != client.ID() {
		oauth2.WriteError(w, oauth2.InvalidGrant(req.State, "Invalid refresh token ownership"))
		return
	}

	// inherit scope from stored refresh token
	if req.Scope.Empty() {
		req.Scope = data.Scope
	}

	// validate scope - a missing scope is always included
	if !data.Scope.Includes(req.Scope) {
		oauth2.WriteError(w, oauth2.InvalidScope(req.State, "Scope exceeds the originally granted scope"))
		return
	}

	// issue tokens
	res, err := a.issueTokens(true, req.Scope, req.State, client.ID(), data.ResourceOwnerID)
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// delete refresh token
	err = a.Storage.DeleteRefreshToken(refreshToken.SignatureString())
	if err != nil {
		oauth2.RedirectError(w, req.RedirectURI, true, err)
	}

	// write response
	oauth2.WriteTokenResponse(w, res)
}

func (a *Authenticator) issueTokens(issueRefreshToken bool, scope oauth2.Scope, state string, clientID bson.ObjectId, resourceOwnerID *bson.ObjectId) (*oauth2.TokenResponse, error) {
	// generate new access token
	accessToken, err := hmacsha.Generate(a.Policy.Secret, 32)
	if err != nil {
		return nil, err
	}

	// generate new refresh token
	refreshToken, err := hmacsha.Generate(a.Policy.Secret, 32)
	if err != nil {
		return nil, err
	}

	// prepare response
	res := bearer.NewTokenResponse(accessToken.String(), int(a.Policy.AccessTokenLifespan/time.Second))

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
		ExpiresAt:       time.Now().Add(a.Policy.AccessTokenLifespan),
		ClientID:        clientID,
		ResourceOwnerID: resourceOwnerID,
	}

	// save access token
	_, err = a.Storage.SaveAccessToken(accessTokenData)
	if err != nil {
		return nil, err
	}

	if issueRefreshToken {
		// create refresh token data
		refreshTokenData := &TokenData{
			Signature:       refreshToken.SignatureString(),
			Scope:           scope,
			ExpiresAt:       time.Now().Add(a.Policy.RefreshTokenLifespan),
			ClientID:        clientID,
			ResourceOwnerID: resourceOwnerID,
		}

		// save refresh token
		_, err := a.Storage.SaveRefreshToken(refreshTokenData)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}
