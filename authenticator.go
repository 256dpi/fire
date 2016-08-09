package fire

import (
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

// AccessToken is the internal model used to store access tokens. The model
// can be mounted as a fire Resource to become manageable via the API.
type AccessToken struct {
	Base          `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Signature     string    `json:"-" valid:"required"`
	RequestedAt   time.Time `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string  `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
}

var accessTokenModel *AccessToken

func init() {
	accessTokenModel = &AccessToken{}
	Init(accessTokenModel)
}

// A Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials, Client Credentials and
// Implicit Grant flows. The flows can be enabled using their respective methods.
type Authenticator struct {
	storage *authenticatorStorage

	provider     *fosite.Fosite
	strategy     *strategy.HMACSHAStrategy
	handleHelper *core.HandleHelper
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
	storage := &authenticatorStorage{
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
	tokenStrategy := &strategy.HMACSHAStrategy{
		Enigma: &hmac.HMACStrategy{
			GlobalSecret: []byte(secret),
		},
		AccessTokenLifespan:   tokenLifespan,
		AuthorizeCodeLifespan: tokenLifespan,
	}

	// create provider
	provider := fosite.NewFosite(storage)

	// set mandatory scope
	provider.MandatoryScope = "fire"

	// this little helper is used by some of the handlers later
	handleHelper := &core.HandleHelper{
		AccessTokenStrategy: tokenStrategy,
		AccessTokenStorage:  storage,
		AccessTokenLifespan: tokenLifespan,
	}

	// add a request validator for access tokens
	provider.AuthorizedRequestValidators.Append(&core.CoreValidator{
		AccessTokenStrategy: tokenStrategy,
		AccessTokenStorage:  storage,
	})

	return &Authenticator{
		storage:      storage,
		provider:     provider,
		handleHelper: handleHelper,
		strategy:     tokenStrategy,
	}
}

// EnablePasswordGrant enables the usage of the OAuth 2.0 Resource Owner Password
// Credentials Grant.
//
// Note: This method should only be called once.
func (a *Authenticator) EnablePasswordGrant() {
	// create handler
	handler := &owner.ResourceOwnerPasswordCredentialsGrantHandler{
		HandleHelper:                                 a.handleHelper,
		ResourceOwnerPasswordCredentialsGrantStorage: a.storage,
	}

	// add handler
	a.provider.TokenEndpointHandlers.Append(handler)
}

// EnableCredentialsGrant enables the usage of the OAuth 2.0 Client Credentials Grant.
//
// Note: This method should only be called once.
func (a *Authenticator) EnableCredentialsGrant() {
	// create handler
	handler := &client.ClientCredentialsGrantHandler{
		HandleHelper: a.handleHelper,
	}

	// add handler
	a.provider.TokenEndpointHandlers.Append(handler)
}

// EnableImplicitGrant enables the usage of the OAuth 2.0 Implicit Grant.
//
// Note: This method should only be called once.
func (a *Authenticator) EnableImplicitGrant() {
	// create handler
	handler := &implicit.AuthorizeImplicitGrantTypeHandler{
		AccessTokenStrategy: a.handleHelper.AccessTokenStrategy,
		AccessTokenStorage:  a.handleHelper.AccessTokenStorage,
		AccessTokenLifespan: a.handleHelper.AccessTokenLifespan,
	}

	// add handler
	a.provider.AuthorizeEndpointHandlers.Append(handler)
}

// HashPassword returns an Authenticator compatible hash of the password.
func (a *Authenticator) HashPassword(password string) ([]byte, error) {
	return a.provider.Hasher.Hash([]byte(password))
}

// MustHashPassword is the same as HashPassword except that it panics when the
// hashing failed.
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
}

// Authorizer returns a callback that can be used to protect resources by requiring
// an access tokens with the provides scopes to be granted.
func (a *Authenticator) Authorizer() Callback {
	// TODO: Add scopes.

	return func(ctx *Context) (error, error) {
		// create new auth context
		authCtx := fosite.NewContext()
		session := &strategy.HMACSession{}

		// validate request
		_, err := a.provider.ValidateRequestAuthorization(authCtx, ctx.GinContext.Request, session, "fire")
		if err != nil {
			return err, nil
		}

		return nil, nil
	}
}

func (a *Authenticator) tokenEndpoint(ctx *gin.Context) {
	// create new auth context
	authCtx := fosite.NewContext()

	// create new session
	s := &strategy.HMACSession{}

	// obtain access request
	req, err := a.provider.NewAccessRequest(authCtx, ctx.Request, s)
	if err != nil {
		ctx.Error(err)
		a.provider.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// grant the mandatory scope
	if req.GetScopes().Has("fire") {
		req.GrantScope("fire")
	}

	// obtain access response
	res, err := a.provider.NewAccessResponse(authCtx, ctx.Request, req)
	if err != nil {
		ctx.Error(err)
		a.provider.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// write response
	a.provider.WriteAccessResponse(ctx.Writer, req, res)
}

func (a *Authenticator) authorizeEndpoint(ctx *gin.Context) {
	// create new auth context
	authCtx := fosite.NewContext()

	// obtain authorize request
	req, err := a.provider.NewAuthorizeRequest(authCtx, ctx.Request)
	if err != nil {
		ctx.Error(err)
		a.provider.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// get credentials
	username := ctx.Request.Form.Get("username")
	password := ctx.Request.Form.Get("password")

	// authenticate user
	err = a.storage.Authenticate(authCtx, username, password)
	if err != nil {
		a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrAccessDenied)
		return
	}

	// create new session
	s := &strategy.HMACSession{}

	// obtain authorize response
	res, err := a.provider.NewAuthorizeResponse(ctx, ctx.Request, req, s)
	if err != nil {
		ctx.Error(err)
		a.provider.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// write response
	a.provider.WriteAuthorizeResponse(ctx.Writer, req, res)
}
