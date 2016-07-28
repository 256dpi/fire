package fire

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kr/pretty"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/handler/core"
	"github.com/ory-am/fosite/handler/core/client"
	"github.com/ory-am/fosite/handler/core/implicit"
	"github.com/ory-am/fosite/handler/core/owner"
	"github.com/ory-am/fosite/handler/core/strategy"
	"github.com/ory-am/fosite/token/hmac"
	"gopkg.in/mgo.v2"
)

var callbackTemplate = []byte(`<!DOCTYPE html>
<html>
  <head>
    <title>Authorize</title>
    <script>
      var hash = window.location.hash;
      window.opener.App.oauth.trigger('redirect', hash);
      window.close();
    </script>
  </head>
</html>`)

type AccessToken struct {
	Base          `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Signature     string    `json:"-" valid:"required"`
	RequestedAt   time.Time `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string  `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
}

type Authenticator struct {
	storage *authenticatorStorage

	strategy     *strategy.HMACSHAStrategy
	handleHelper *core.HandleHelper
	fosite       *fosite.Fosite
}

var accessTokenModel *AccessToken

func init() {
	accessTokenModel = &AccessToken{}
	Init(accessTokenModel)
}

func NewAuthenticator(db *mgo.Database, ownerModel, clientModel Model, secret string) *Authenticator {
	// initialize models
	Init(ownerModel)
	Init(clientModel)

	// extract attributes from owner
	ownerIdentifiable := ownerModel.getBase().attributesByTag("identifiable")
	if len(ownerIdentifiable) != 1 {
		panic("expected to find exactly one 'identifiable' tag on model")
	}
	ownerVerifiable := ownerModel.getBase().attributesByTag("verifiable")
	if len(ownerVerifiable) != 1 {
		panic("expected to find exactly one 'verifiable' tag on model")
	}

	// extract attributes from client
	clientIdentifiable := clientModel.getBase().attributesByTag("identifiable")
	if len(clientIdentifiable) != 1 {
		panic("expected to find exactly one 'identifiable' tag on model")
	}
	clientVerifiable := clientModel.getBase().attributesByTag("verifiable")
	if len(clientVerifiable) != 1 {
		panic("expected to find exactly one 'verifiable' tag on model")
	}

	// create storage
	s := &authenticatorStorage{
		db:               db,
		ownerModel:       ownerModel,
		ownerIDAttr:      ownerIdentifiable[0],
		ownerSecretAttr:  ownerVerifiable[0],
		clientModel:      clientModel,
		clientIDAttr:     clientIdentifiable[0],
		clientSecretAttr: clientVerifiable[0],
	}

	// set the default token lifespan to one hour
	tokenLifespan := time.Hour

	// create a new token generation strategy
	strategy := &strategy.HMACSHAStrategy{
		Enigma: &hmac.HMACStrategy{
			GlobalSecret: []byte(secret),
		},
		AccessTokenLifespan:   tokenLifespan,
		AuthorizeCodeLifespan: tokenLifespan,
	}

	// instantiate a new fosite instance
	f := fosite.NewFosite(s)

	// set mandatory scope
	f.MandatoryScope = "fire"

	// this little helper is used by some of the handlers later
	handleHelper := &core.HandleHelper{
		AccessTokenStrategy: strategy,
		AccessTokenStorage:  s,
		AccessTokenLifespan: tokenLifespan,
	}

	// TODO: Enable refresh token.
	// this handler is responsible for the refresh token grant
	//refreshHandler := &refresh.RefreshTokenGrantHandler{
	//	AccessTokenStrategy:      strategy,
	//	RefreshTokenStrategy:     strategy,
	//	RefreshTokenGrantStorage: s,
	//	AccessTokenLifespan:      accessTokenLifespan,
	//}

	// add handler to fosite
	//f.TokenEndpointHandlers.Append(refreshHandler)

	// add a request validator for access tokens to fosite
	f.AuthorizedRequestValidators.Append(&core.CoreValidator{
		AccessTokenStrategy: strategy,
		AccessTokenStorage:  s,
	})

	return &Authenticator{
		storage:      s,
		fosite:       f,
		handleHelper: handleHelper,
		strategy:     strategy,
	}
}

// EnablePasswordGrant enables the usage of the OAuth 2.0 Resource Owner Password
// Credentials Grant.
func (a *Authenticator) EnablePasswordGrant() {
	// create handler
	passwordHandler := &owner.ResourceOwnerPasswordCredentialsGrantHandler{
		HandleHelper:                                 a.handleHelper,
		ResourceOwnerPasswordCredentialsGrantStorage: a.storage,
	}

	// add handler to fosite
	a.fosite.TokenEndpointHandlers.Append(passwordHandler)
}

// EnableCredentialsGrant enables the usage of the OAuth 2.0 Client Credentials Grant.
func (a *Authenticator) EnableCredentialsGrant() {
	// create handler
	credentialsHandler := &client.ClientCredentialsGrantHandler{
		HandleHelper: a.handleHelper,
	}

	// add handler to fosite
	a.fosite.TokenEndpointHandlers.Append(credentialsHandler)
}

// EnableImplicitGrant enables the usage of the OAuth 2.0 Implicit Grant.
func (a *Authenticator) EnableImplicitGrant() {
	// create handler
	implicitHandler := &implicit.AuthorizeImplicitGrantTypeHandler{
		AccessTokenStrategy: a.handleHelper.AccessTokenStrategy,
		AccessTokenStorage:  a.handleHelper.AccessTokenStorage,
		AccessTokenLifespan: a.handleHelper.AccessTokenLifespan,
	}

	// add handler to fosite
	a.fosite.AuthorizeEndpointHandlers.Append(implicitHandler)
}

// HashPassword returns an Authenticator compatible hash of the password.
func (a *Authenticator) HashPassword(password string) ([]byte, error) {
	return a.fosite.Hasher.Hash([]byte(password))
}

// MustHashPassword is the same as HashPassword except that it raises and error
// when the hashing failed.
func (a *Authenticator) MustHashPassword(password string) []byte {
	bytes, err := a.HashPassword(password)
	if err != nil {
		panic(err)
	}

	return bytes
}

func (a *Authenticator) Register(prefix string, router gin.IRouter) {
	router.POST(prefix+"/token", a.tokenEndpoint)
	router.GET(prefix+"/authorize", a.authorizeEndpoint)
	router.POST(prefix+"/authorize", a.authorizeEndpoint)
	router.GET(prefix+"/callback", a.callbackEndpoint)
}

func (a *Authenticator) Authorizer() Callback {
	return func(ctx *Context) (error, error) {
		// prepare fosite
		f := fosite.NewContext()
		session := &strategy.HMACSession{}

		// validate request
		_, err := a.fosite.ValidateRequestAuthorization(f, ctx.GinContext.Request, session, "fire")
		if err != nil {
			return err, nil
		}

		return nil, nil
	}
}

func (a *Authenticator) tokenEndpoint(ctx *gin.Context) {
	// create new context
	f := fosite.NewContext()

	// create new session
	s := &strategy.HMACSession{}

	// obtain access request
	req, err := a.fosite.NewAccessRequest(f, ctx.Request, s)
	if err != nil {
		pretty.Println(err)
		a.fosite.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// grant the mandatory scope
	if req.GetScopes().Has("fire") {
		req.GrantScope("fire")
	}

	// obtain access response
	res, err := a.fosite.NewAccessResponse(f, ctx.Request, req)
	if err != nil {
		pretty.Println(err)
		a.fosite.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// write response
	a.fosite.WriteAccessResponse(ctx.Writer, req, res)
}

func (a *Authenticator) authorizeEndpoint(ctx *gin.Context) {
	// create new context
	f := fosite.NewContext()

	// obtain authorize request
	req, err := a.fosite.NewAuthorizeRequest(f, ctx.Request)
	if err != nil {
		pretty.Println(err)
		a.fosite.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}
	// You have now access to authorizeRequest, Code ResponseTypes, Scopes ...

	// Normally, this would be the place where you would check if the user is logged in and gives his consent.
	// We're simplifying things and just checking if the request includes a valid username and password
	if ctx.Request.Form.Get("username") != "peter" {
		ctx.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		ctx.Writer.Write([]byte(`<h1>Login page</h1>`))
		ctx.Writer.Write([]byte(`
			<p>Howdy! This is the log in page. For this example, it is enough to supply the username.</p>
			<form method="post">
				<input type="text" name="username" /> <small>try peter</small><br>
				<input type="submit">
			</form>
		`))
		return
	}

	// we allow issuing of refresh tokens per default
	if req.GetScopes().Has("offline") {
		req.GrantScope("offline")
	}

	// create new session
	s := &strategy.HMACSession{}

	// obtain authorize response
	res, err := a.fosite.NewAuthorizeResponse(ctx, ctx.Request, req, s)
	if err != nil {
		pretty.Println(err)
		a.fosite.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// write response
	a.fosite.WriteAuthorizeResponse(ctx.Writer, req, res)
}

func (a *Authenticator) callbackEndpoint(ctx *gin.Context) {
	ctx.Writer.Write(callbackTemplate)
}
