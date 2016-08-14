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

// A GrantRequest is used in conjunction with the GrantStrategy.
type GrantRequest struct {
	GrantType       string
	RequestedScopes []string
	Client          Model
	Owner           Model
}

// The GrantStrategy is invoked by the Authenticator with the grant type,
// requested scopes, the client and the owner before issuing an AccessToken.
// The callback should return a list of additional scopes that should be granted.
//
// Note: The Owner is not set for a client credentials grant.
type GrantStrategy func(req *GrantRequest) []string

// DefaultGrantStrategy grants all requested scopes.
func DefaultGrantStrategy(req *GrantRequest) []string {
	return req.RequestedScopes
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

// TODO: Rename to a more matching name?

// AccessToken is the internal model used to store access tokens. The model
// can be mounted as a fire Resource to become manageable via the API.
type AccessToken struct {
	Base          `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Type          string         `json:"type"`
	Signature     string         `json:"signature" valid:"required"`
	RequestedAt   time.Time      `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string       `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
	ClientID      bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id" fire:"filterable,sortable"`
	OwnerID       *bson.ObjectId `json:"owner-id" valid:"-" bson:"owner_id" fire:"filterable,sortable"`
}

// A Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials, Client Credentials and
// Implicit Grant flows. The flows can be enabled using their respective methods.
type Authenticator struct {
	GrantStrategy   GrantStrategy
	CompareCallback CompareCallback

	enabledGrants   []string

	config          *compose.Config
	provider        *fosite.Fosite
	strategy        *oauth2.HMACSHAStrategy
	storage         *authenticatorStorage
}

// TODO: Allow passing a custom AccessToken model.

// NewAuthenticator creates and returns a new Authenticator.
func NewAuthenticator(db *mgo.Database, ownerModel, clientModel Model, secret string) *Authenticator {
	// initialize models
	Init(ownerModel)
	Init(clientModel)

	// prepare access token model
	accessTokenModel := &AccessToken{}
	Init(accessTokenModel)

	// create storage
	storage := &authenticatorStorage{
		db:               db,
		ownerModel:       ownerModel,
		clientModel:      clientModel,
		accessTokenModel: accessTokenModel,
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
		GrantStrategy:   DefaultGrantStrategy,
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
	var owner Model

	// retrieve owner
	if ctx.Request.FormValue("grant_type") == "password" {
		owner, err = a.storage.getOwner(ctx.Request.FormValue("username"))
		if err != nil {
			a.provider.WriteAccessError(ctx.Writer, nil, err)
			return
		}

		// assign owner to context
		ctx.Set("owner", owner)
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
	client := req.GetClient().(*authenticatorClient).model

	// set client
	ctx.Set("client", client)

	// grant additional scopes if the grant callback is present
	a.invokeGrantStrategy(grantType, req, client, owner)

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
	owner, err := a.storage.getOwner(username)
	if err != nil {
		a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrAccessDenied)
		return
	}

	// assign owner to context
	ctx.Set("owner", owner)

	// authenticate user
	err = a.storage.Authenticate(ctx, username, password)
	if err != nil {
		a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrAccessDenied)
		return
	}

	// retrieve client
	client := req.GetClient().(*authenticatorClient).model

	// set client
	ctx.Set("client", client)

	// check if client has all scopes
	for _, scope := range req.GetRequestedScopes() {
		if !a.provider.ScopeStrategy(req.GetClient().GetScopes(), scope) {
			a.provider.WriteAuthorizeError(ctx.Writer, req, fosite.ErrInvalidScope)
			return
		}
	}

	// grant additional scopes if the grant callback is present
	a.invokeGrantStrategy("implicit", req, client, owner)

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

func (a *Authenticator) invokeGrantStrategy(grantType string, req fosite.Requester, client, owner Model) {
	if a.GrantStrategy != nil {
		grantedScopes := a.GrantStrategy(&GrantRequest{
			GrantType:       grantType,
			RequestedScopes: req.GetRequestedScopes(),
			Client:          client,
			Owner:           owner,
		})

		for _, scope := range grantedScopes {
			req.GrantScope(scope)
		}
	}
}

func (a *Authenticator) isFatalError(err error) bool {
	return fosite.ErrorToRFC6749Error(err).StatusCode == http.StatusInternalServerError
}
