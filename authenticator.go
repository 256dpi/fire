package fire

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
      window.opener.fireCallback(window.location.hash);
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

// NetAuthenticator creates and returns a new Authenticator.
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
	clientCallable := clientModel.getBase().attributesByTag("callable")
	if len(clientCallable) != 1 {
		panic("expected to find exactly one 'callable' tag on model")
	}

	// create storage
	s := &authenticatorStorage{
		db:                 db,
		ownerModel:         ownerModel,
		ownerIDAttr:        ownerIdentifiable[0],
		ownerSecretAttr:    ownerVerifiable[0],
		clientModel:        clientModel,
		clientIDAttr:       clientIdentifiable[0],
		clientSecretAttr:   clientVerifiable[0],
		clientCallableAttr: clientCallable[0],
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
//
// Note: This method should only be called once.
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
//
// Note: This method should only be called once.
func (a *Authenticator) EnableCredentialsGrant() {
	// create handler
	credentialsHandler := &client.ClientCredentialsGrantHandler{
		HandleHelper: a.handleHelper,
	}

	// add handler to fosite
	a.fosite.TokenEndpointHandlers.Append(credentialsHandler)
}

// EnableImplicitGrant enables the usage of the OAuth 2.0 Implicit Grant.
//
// Note: This method should only be called once.
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

// Register will create all necessary routes on the passed router. If want to
// prefix the auth endpoint (e.g. /auth/) you need to pass it to Register.
//
// Note: This functions should only be called once after enabling all flows.
func (a *Authenticator) Register(prefix string, router gin.IRouter) {
	router.POST(prefix+"/token", a.tokenEndpoint)
	router.POST(prefix+"/authorize", a.authorizeEndpoint)
	router.GET(prefix+"/callback", a.callbackEndpoint)

	// TODO: Redirect to auxiliary Login form.
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
		a.fosite.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// get credentials
	username := ctx.Request.Form.Get("username")
	password := ctx.Request.Form.Get("password")

	// authenticate user
	err = a.storage.Authenticate(f, username, password)
	if err != nil {
		uri := ctx.Request.Referer() + "&error=invalid_credentials"
		ctx.Redirect(http.StatusTemporaryRedirect, uri)
		return
	}

	// create new session
	s := &strategy.HMACSession{}

	// obtain authorize response
	res, err := a.fosite.NewAuthorizeResponse(ctx, ctx.Request, req, s)
	if err != nil {
		a.fosite.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// write response
	a.fosite.WriteAuthorizeResponse(ctx.Writer, req, res)
}

func (a *Authenticator) callbackEndpoint(ctx *gin.Context) {
	ctx.Writer.Write(callbackTemplate)
}
