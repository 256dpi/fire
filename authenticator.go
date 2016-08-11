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
	"gopkg.in/mgo.v2/bson"
)

// The GrantCallback is invoked by the Authenticator with the grant type,
// requested scopes, the client and the owner before issuing an AccessToken.
// The callback should return a list of additional scopes that should be granted.
//
// Note: The Owner is not set for a client credentials grant.
type GrantCallback func(grant string, scopes []string, client Model, owner Model) []string

// AccessToken is the internal model used to store access tokens. The model
// can be mounted as a fire Resource to become manageable via the API.
type AccessToken struct {
	Base          `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Signature     string         `json:"-" valid:"required"`
	RequestedAt   time.Time      `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string       `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
	ClientID      bson.ObjectId  `json:"-" valid:"-" bson:"client_id"`
	OwnerID       *bson.ObjectId `json:"-" valid:"-" bson:"owner_id"`
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
	GrantCallback GrantCallback

	storage *authenticatorStorage

	provider     *fosite.Fosite
	strategy     *strategy.HMACSHAStrategy
	handleHelper *core.HandleHelper
}

// NetAuthenticator creates and returns a new Authenticator.
func NewAuthenticator(db *mgo.Database, ownerModel, clientModel Model, secret, mandatoryScope string) *Authenticator {
	// initialize models
	Init(ownerModel)
	Init(clientModel)

	// extract attributes from owner
	ownerIdentifiable := ownerModel.getBase().attributesByTag("identifiable")
	if len(ownerIdentifiable) != 1 {
		panic("expected to find exactly one 'identifiable' attribute on model")
	}
	ownerVerifiable := ownerModel.getBase().attributesByTag("verifiable")
	if len(ownerVerifiable) != 1 {
		panic("expected to find exactly one 'verifiable' attribute on model")
	}

	// extract attributes from client
	clientIdentifiable := clientModel.getBase().attributesByTag("identifiable")
	if len(clientIdentifiable) != 1 {
		panic("expected to find exactly one 'identifiable' attribute on model")
	}
	clientVerifiable := clientModel.getBase().attributesByTag("verifiable")
	if len(clientVerifiable) != 1 {
		panic("expected to find exactly one 'verifiable' attribute on model")
	}
	clientCallable := clientModel.getBase().attributesByTag("callable")
	if len(clientCallable) != 1 {
		panic("expected to find exactly one 'callable' attribute on model")
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
	provider.MandatoryScope = mandatoryScope

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
// an access tokens with the provided scopes to be granted.
func (a *Authenticator) Authorizer(scopes ...string) Callback {
	return func(ctx *Context) (error, error) {
		// create new session
		session := &strategy.HMACSession{}

		// add mandatory scope if missing
		if !stringInList(scopes, a.provider.MandatoryScope) {
			scopes = append(scopes, a.provider.MandatoryScope)
		}

		// validate request
		_, err := a.provider.ValidateRequestAuthorization(ctx.GinContext, ctx.GinContext.Request, session, scopes...)
		if err != nil {
			return err, nil
		}

		// TODO: Assign client to context.

		return nil, nil
	}
}

func (a *Authenticator) tokenEndpoint(ctx *gin.Context) {
	var err error

	// create new session
	session := &strategy.HMACSession{}

	// prepare optional owner
	var ownerModel Model

	// retrieve owner
	if ctx.Request.FormValue("grant_type") == "password" {
		ownerModel, err = a.storage.getOwner(ctx.Request.FormValue("username"))
		if err != nil {
			a.provider.WriteAccessError(ctx.Writer, nil, fosite.ErrInvalidRequest)
			return
		}

		// assign owner to context
		ctx.Set("owner", ownerModel)
	}

	// obtain access request
	req, err := a.provider.NewAccessRequest(ctx, ctx.Request, session)
	if err != nil {
		if a.isFatalError(err) {
			ctx.Error(err)
		}

		a.provider.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// extract grant type
	grantType := req.GetGrantTypes()[0]

	// retrieve client
	clientModel := req.GetClient().(*authenticatorClient).model

	// set client
	ctx.Set("client", clientModel)

	// grant additional scopes if the grant callback is present
	a.invokeGrantCallback(grantType, req, clientModel, ownerModel)

	// obtain access response
	res, err := a.provider.NewAccessResponse(ctx, ctx.Request, req)
	if err != nil {
		if a.isFatalError(err) {
			ctx.Error(err)
		}

		a.provider.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// write response
	a.provider.WriteAccessResponse(ctx.Writer, req, res)
}

func (a *Authenticator) authorizeEndpoint(ctx *gin.Context) {
	// obtain authorize request
	req, err := a.provider.NewAuthorizeRequest(ctx, ctx.Request)
	if err != nil {
		if a.isFatalError(err) {
			ctx.Error(err)
		}

		a.provider.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// get credentials
	username := ctx.Request.FormValue("username")
	password := ctx.Request.FormValue("password")

	// retrieve owner
	ownerModel, err := a.storage.getOwner(username)
	if err != nil {
		a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrAccessDenied)
		return
	}

	// assign owner to context
	ctx.Set("owner", ownerModel)

	// authenticate user
	err = a.storage.Authenticate(ctx, username, password)
	if err != nil {
		a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrAccessDenied)
		return
	}

	// retrieve client
	clientModel := req.GetClient().(*authenticatorClient).model

	// set client
	ctx.Set("client", clientModel)

	// grant additional scopes if the grant callback is present
	a.invokeGrantCallback("implicit", req, clientModel, ownerModel)

	// create new session
	session := &strategy.HMACSession{}

	// obtain authorize response
	res, err := a.provider.NewAuthorizeResponse(ctx, ctx.Request, req, session)
	if err != nil {
		if a.isFatalError(err) {
			ctx.Error(err)
		}

		a.provider.WriteAuthorizeError(ctx.Writer, req, err)
		return
	}

	// write response
	a.provider.WriteAuthorizeResponse(ctx.Writer, req, res)
}

func (a *Authenticator) invokeGrantCallback(grantType string, req fosite.Requester, clientModel, ownerModel Model) {
	if a.GrantCallback != nil {
		additionalScopes := make([]string, 0)
		for _, scope := range req.GetScopes() {
			if scope != a.provider.MandatoryScope {
				additionalScopes = append(additionalScopes, scope)
			}
		}

		for _, scope := range a.GrantCallback(grantType, additionalScopes, clientModel, ownerModel) {
			req.GrantScope(scope)
		}
	}
}

func (a *Authenticator) isFatalError(err error) bool {
	return fosite.ErrorToRFC6749Error(err).StatusCode == http.StatusInternalServerError
}
