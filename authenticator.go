package fire

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/compose"
	"github.com/ory-am/fosite/handler/oauth2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// the default hash cost that is used by the token hasher
var hashCost = bcrypt.DefaultCost

// The GrantCallback is invoked by the Authenticator with the grant type,
// requested scopes, the client and the owner before issuing an AccessToken.
// The callback should return a list of additional scopes that should be granted.
//
// Note: The Owner is not set for a client credentials grant.
type GrantCallback func(grant string, scopes []string, client Model, owner Model) []string

// DefaultGrantCallback grants all requested scopes.
func DefaultGrantCallback(_ string, scopes []string, _ Model, _ Model) []string {
	return scopes
}

// The CompareCallback is invoked by the Authenticator with the stored password
// hash and submitted password of a owner. The callback is responsible for
// comparing the submitted password with the stored hash and should return an
// error if they do not match.
type CompareCallback func(hash, password []byte) error

// DefaultCompareCallback uses bcrypt to compare the hash and the password.
func DefaultCompareCallback(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

// AccessToken is the internal model used to store access tokens. The model
// can be mounted as a fire Resource to become manageable via the API.
type AccessToken struct {
	Base          `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Signature     string         `json:"-" valid:"required"`
	RequestedAt   time.Time      `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string       `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
	ClientID      bson.ObjectId  `json:"-" valid:"-" bson:"client_id" fire:"filterable,sortable"`
	OwnerID       *bson.ObjectId `json:"-" valid:"-" bson:"owner_id" fire:"filterable,sortable"`
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
	GrantCallback   GrantCallback
	CompareCallback CompareCallback

	enabledGrants []string

	config   *compose.Config
	provider *fosite.Fosite
	strategy *oauth2.HMACSHAStrategy
	storage  *authenticatorStorage
}

// NewAuthenticator creates and returns a new Authenticator.
func NewAuthenticator(db *mgo.Database, ownerModel, clientModel Model, secret string) *Authenticator {
	// initialize models
	Init(ownerModel)
	Init(clientModel)

	// extract attributes from owner
	ownerIdentifiable := ownerModel.getBase().attributesByTag("identifiable")
	if len(ownerIdentifiable) != 1 {
		panic("Expected to find exactly one 'identifiable' attribute on the passed owner model")
	}
	ownerVerifiable := ownerModel.getBase().attributesByTag("verifiable")
	if len(ownerVerifiable) != 1 {
		panic("Expected to find exactly one 'verifiable' attribute on the passed owner model")
	}

	// extract attributes from client
	clientIdentifiable := clientModel.getBase().attributesByTag("identifiable")
	if len(clientIdentifiable) != 1 {
		panic("Expected to find exactly one 'identifiable' attribute on the passed client model")
	}
	clientVerifiable := clientModel.getBase().attributesByTag("verifiable")
	if len(clientVerifiable) != 1 {
		panic("Expected to find exactly one 'verifiable' attribute on the passed client model")
	}
	clientGrantable := clientModel.getBase().attributesByTag("grantable")
	if len(clientGrantable) != 1 {
		panic("Expected to find exactly one 'grantable' attribute on the passed client model")
	}
	clientCallable := clientModel.getBase().attributesByTag("callable")
	if len(clientCallable) != 1 {
		panic("Expected to find exactly one 'callable' attribute on the passed client model")
	}

	// create storage
	storage := &authenticatorStorage{
		db:                  db,
		ownerModel:          ownerModel,
		ownerIDAttr:         ownerIdentifiable[0],
		ownerSecretAttr:     ownerVerifiable[0],
		clientModel:         clientModel,
		clientIDAttr:        clientIdentifiable[0],
		clientSecretAttr:    clientVerifiable[0],
		clientGrantableAttr: clientGrantable[0],
		clientCallableAttr:  clientCallable[0],
	}

	// provider config
	config := &compose.Config{
		HashCost: hashCost,
	}

	// create a new token generation strategy
	strategy := compose.NewOAuth2HMACStrategy(config, []byte(secret))

	// create provider
	provider := compose.Compose(config, storage, strategy)

	// TODO: Implement refresh tokens.
	// TODO: Allow enabling access code flow (explicit)?

	// create authenticator
	a := &Authenticator{
		GrantCallback:   DefaultGrantCallback,
		CompareCallback: DefaultCompareCallback,

		config:   config,
		provider: provider.(*fosite.Fosite),
		strategy: strategy,
		storage:  storage,
	}

	// set authenticator on storage
	storage.authenticator = a

	return a
}

// EnablePasswordGrant enables the usage of the OAuth 2.0 Resource Owner Password
// Credentials Grant.
func (a *Authenticator) EnablePasswordGrant() {
	if stringInList(a.enabledGrants, "password") {
		panic("The password grant has already been enabled")
	}

	// create and register handler
	grantHandler := compose.OAuth2ResourceOwnerPasswordCredentialsFactory(a.config, a.storage, a.strategy)
	a.provider.TokenEndpointHandlers.Append(grantHandler.(fosite.TokenEndpointHandler))
	a.provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))

	a.enabledGrants = append(a.enabledGrants, "password")
}

// EnableCredentialsGrant enables the usage of the OAuth 2.0 Client Credentials Grant.
func (a *Authenticator) EnableCredentialsGrant() {
	if stringInList(a.enabledGrants, "client_credentials") {
		panic("The client credentials grant has already been enabled")
	}

	// create and register handler
	grantHandler := compose.OAuth2ClientCredentialsGrantFactory(a.config, a.storage, a.strategy)
	a.provider.TokenEndpointHandlers.Append(grantHandler.(fosite.TokenEndpointHandler))
	a.provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))

	a.enabledGrants = append(a.enabledGrants, "client_credentials")
}

// EnableImplicitGrant enables the usage of the OAuth 2.0 Implicit Grant.
func (a *Authenticator) EnableImplicitGrant() {
	if stringInList(a.enabledGrants, "implicit") {
		panic("The implicit grant has already been enabled")
	}

	// create and register handler
	grantHandler := compose.OAuth2AuthorizeImplicitFactory(a.config, a.storage, a.strategy)
	a.provider.AuthorizeEndpointHandlers.Append(grantHandler.(fosite.AuthorizeEndpointHandler))
	a.provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))

	a.enabledGrants = append(a.enabledGrants, "implicit")
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
	if len(scopes) < 1 {
		panic("Authorizer must be called with at least one scope")
	}

	return func(ctx *Context) error {
		// create new session
		session := &oauth2.HMACSession{}

		// get token
		token := fosite.AccessTokenFromRequest(ctx.GinContext.Request)

		// validate request
		_, err := a.provider.ValidateToken(ctx.GinContext, token, fosite.AccessToken, session, scopes...)
		if err != nil {
			return err
		}

		return nil
	}
}

func (a *Authenticator) tokenEndpoint(ctx *gin.Context) {
	var err error

	// create new session
	session := &oauth2.HMACSession{}

	// prepare optional owner
	var ownerModel Model

	// retrieve owner
	if ctx.Request.FormValue("grant_type") == "password" {
		ownerModel, err = a.storage.getOwner(ctx.Request.FormValue("username"))
		if err != nil {
			a.provider.WriteAccessError(ctx.Writer, nil, err)
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

	// check if client has all scopes
	for _, scope := range req.GetRequestedScopes() {
		if !a.provider.ScopeStrategy(req.GetClient().GetScopes(), scope) {
			a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrInvalidScope)
			return
		}
	}

	// grant additional scopes if the grant callback is present
	a.invokeGrantCallback("implicit", req, clientModel, ownerModel)

	// create new session
	session := &oauth2.HMACSession{}

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
		for _, scope := range a.GrantCallback(grantType, req.GetRequestedScopes(), clientModel, ownerModel) {
			req.GrantScope(scope)
		}
	}
}

func (a *Authenticator) isFatalError(err error) bool {
	return fosite.ErrorToRFC6749Error(err).StatusCode == http.StatusInternalServerError
}
