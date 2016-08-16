package fire

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imdario/mergo"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/compose"
	"github.com/ory-am/fosite/handler/oauth2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// the default hash cost that is used by the token hasher
var hashCost = bcrypt.DefaultCost

const (
	// PasswordGrant specifies the usage of the OAuth 2.0 Resource Owner Password
	// Credentials Grant.
	PasswordGrant = "password"

	// ClientCredentialsGrant specifies the usage of the OAuth 2.0 Client
	// Credentials Grant.
	ClientCredentialsGrant = "client_credentials"

	// ImplicitGrant specifies the usage of the OAuth 2.0 Implicit Grant.
	ImplicitGrant = "implicit"
)

// A GrantRequest is used in conjunction with the GrantStrategy.
type GrantRequest struct {
	GrantType       []string
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

// The CompareStrategy is invoked by the Authenticator with the stored password
// hash and submitted password of a owner. The callback is responsible for
// comparing the submitted password with the stored hash and should return an
// error if they do not match.
type CompareStrategy func(hash, password []byte) error

// DefaultCompareStrategy uses bcrypt to compare the hash and the password.
func DefaultCompareStrategy(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

// AccessToken is the built-in model used to store access tokens. The model
// can be mounted as a fire Resource to become manageable via the API.
type AccessToken struct {
	Base          `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Signature     string         `json:"signature" valid:"required"`
	RequestedAt   time.Time      `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string       `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
	ClientID      *bson.ObjectId `json:"client-id" valid:"-" bson:"client_id" fire:"filterable,sortable"`
	OwnerID       *bson.ObjectId `json:"owner-id" valid:"-" bson:"owner_id" fire:"filterable,sortable"`
}

// Application is the built-in model used to store clients. The model can be
// mounted as a fire Resource to become manageable via the API.
type Application struct {
	Base       `bson:",inline" fire:"application:applications"`
	Name       string   `json:"name" valid:"required"`
	Key        string   `json:"key" valid:"required"`
	Secret     []byte   `json:"secret" valid:"required"`
	Scopes     []string `json:"scopes" valid:"required"`
	GrantTypes []string `json:"grant-types" valid:"required" bson:"grant_types"`
	Callbacks  []string `json:"callbacks" valid:"required"`
}

// A Policy is used to prepare an authentication policy for an Authenticator.
type Policy struct {
	Secret           []byte
	OwnerModel       Model
	ClientModel      Model
	AccessTokenModel Model
	EnabledGrants    []string
	GrantStrategy    GrantStrategy
	CompareStrategy  CompareStrategy
	TokenLifespan    time.Duration
}

var defaultPolicy = Policy{
	ClientModel:      &Application{},
	AccessTokenModel: &AccessToken{},
	GrantStrategy:    DefaultGrantStrategy,
	CompareStrategy:  DefaultCompareStrategy,
	TokenLifespan:    time.Hour,
}

// An Authenticator provides OAuth2 based authentication. The implementation
// currently supports the Resource Owner Credentials Grant, Client Credentials
// Grant and Implicit Grant flows.
type Authenticator struct {
	db     *mgo.Database
	policy *Policy

	config   *compose.Config
	provider *fosite.Fosite
	strategy *oauth2.HMACSHAStrategy
	storage  *authenticatorStorage
}

// NewAuthenticator creates and returns a new Authenticator.
func NewAuthenticator(db *mgo.Database, policy *Policy) *Authenticator {
	// set defaults
	err := mergo.Merge(policy, defaultPolicy)
	if err != nil {
		panic(err)
	}

	// initialize models
	Init(policy.OwnerModel)
	Init(policy.ClientModel)
	Init(policy.AccessTokenModel)

	// create storage
	storage := &authenticatorStorage{}

	// provider config
	config := &compose.Config{
		AccessTokenLifespan: policy.TokenLifespan,
		HashCost:            hashCost,
	}

	// create a new token generation strategy
	strategy := compose.NewOAuth2HMACStrategy(config, policy.Secret)

	// create provider
	provider := compose.Compose(config, storage, strategy).(*fosite.Fosite)

	// add password grant handler
	if stringInList(policy.EnabledGrants, PasswordGrant) {
		grantHandler := compose.OAuth2ResourceOwnerPasswordCredentialsFactory(config, storage, strategy)
		provider.TokenEndpointHandlers.Append(grantHandler.(fosite.TokenEndpointHandler))
		provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))
	}

	// add client credentials grant handler
	if stringInList(policy.EnabledGrants, ClientCredentialsGrant) {
		grantHandler := compose.OAuth2ClientCredentialsGrantFactory(config, storage, strategy)
		provider.TokenEndpointHandlers.Append(grantHandler.(fosite.TokenEndpointHandler))
		provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))
	}

	// add implicit grant handler
	if stringInList(policy.EnabledGrants, ImplicitGrant) {
		grantHandler := compose.OAuth2AuthorizeImplicitFactory(config, storage, strategy)
		provider.AuthorizeEndpointHandlers.Append(grantHandler.(fosite.AuthorizeEndpointHandler))
		provider.TokenValidators.Append(grantHandler.(fosite.TokenValidator))
	}

	// create authenticator
	a := &Authenticator{
		db:       db,
		policy:   policy,
		config:   config,
		provider: provider,
		strategy: strategy,
		storage:  storage,
	}

	// set authenticator on storage
	storage.authenticator = a

	return a
}

// Register will create all necessary routes on the passed router. If want to
// prefix the auth endpoint (e.g. /auth/) you need to pass it to Register.
//
// Note: This functions should only be called once after enabling all flows.
func (a *Authenticator) Register(prefix string, router gin.IRouter) {
	router.POST(prefix+"/token", a.tokenEndpoint)
	router.POST(prefix+"/authorize", a.authorizeEndpoint)
}

// NewKeyAndSignature returns a new key with a matching signature that can be
// used to issue custom access tokens.
func (a *Authenticator) NewKeyAndSignature() (string, string, error) {
	return a.strategy.GenerateAccessToken(nil, nil)
}

// Authorize can used to authorize a request by requiring an access token with
// the provided scopes to be granted.
func (a *Authenticator) Authorize(ctx *gin.Context, scopes []string) error {
	// create new session
	session := &oauth2.HMACSession{}

	// get token
	token := fosite.AccessTokenFromRequest(ctx.Request)

	// validate request
	_, err := a.provider.ValidateToken(ctx, token, fosite.AccessToken, session, scopes...)
	if err != nil {
		return err
	}

	return nil
}

// Authorizer returns a callback that can be used to protect resources by
// requiring an access token with the provided scopes to be granted.
func (a *Authenticator) Authorizer(scopes ...string) Callback {
	if len(scopes) < 1 {
		panic("Authorizer must be called with at least one scope")
	}

	return func(ctx *Context) error {
		return a.Authorize(ctx.GinContext, scopes)
	}
}

// GinAuthorizer can be used to protect plain handler by requiring an access token
// with the provided scopes to be granted.
func (a *Authenticator) GinAuthorizer(scopes ...string) gin.HandlerFunc {
	if len(scopes) < 1 {
		panic("GinAuthorizer must be called with at least one scope")
	}

	return func(ctx *gin.Context) {
		err := a.Authorize(ctx, scopes)
		if err != nil {
			ctx.AbortWithError(http.StatusUnauthorized, err)
			return
		}

		// call next handler
		ctx.Next()
	}
}

func (a *Authenticator) tokenEndpoint(ctx *gin.Context) {
	var err error

	// create new session
	session := &oauth2.HMACSession{}

	// prepare optional owner
	var owner Model

	// retrieve owner
	if ctx.Request.FormValue("grant_type") == PasswordGrant {
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
	a.invokeGrantStrategy(ImplicitGrant, req, client, owner)

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
	if a.policy.GrantStrategy != nil {
		grantedScopes := a.policy.GrantStrategy(&GrantRequest{
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

type authenticatorStorage struct {
	authenticator *Authenticator
}

type authenticatorClient struct {
	fosite.DefaultClient
	model Model
}

func (s *authenticatorStorage) GetClient(id string) (fosite.Client, error) {
	// prepare object
	obj := newStructPointer(s.authenticator.policy.ClientModel)

	// query db
	err := s.authenticator.db.C(s.authenticator.policy.ClientModel.Meta().Collection).Find(bson.M{
		"key": id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidClient
	} else if err != nil {
		return nil, err
	}

	// initialize model
	client := Init(obj.(Model))

	return &authenticatorClient{
		DefaultClient: fosite.DefaultClient{
			ID:            id,
			Secret:        client.Get("Secret").([]byte),
			GrantTypes:    client.Get("GrantTypes").([]string),
			ResponseTypes: []string{"token"},
			RedirectURIs:  client.Get("Callbacks").([]string),
			Scopes:        client.Get("Scopes").([]string),
		},
		model: client,
	}, nil
}

func (s *authenticatorStorage) CreateAuthorizeCodeSession(ctx context.Context, code string, request fosite.Requester) error {
	return errors.New("not implemented")
}

func (s *authenticatorStorage) GetAuthorizeCodeSession(ctx context.Context, code string, session interface{}) (fosite.Requester, error) {
	return nil, errors.New("not implemented")
}

func (s *authenticatorStorage) DeleteAuthorizeCodeSession(ctx context.Context, code string) error {
	return errors.New("not implemented")
}

func (s *authenticatorStorage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	// retrieve client id
	clientID := ctx.Value("client").(Model).ID()

	// retrieve optional owner id
	var ownerID *bson.ObjectId
	if ctx.Value("owner") != nil {
		id := ctx.Value("owner").(Model).ID()
		ownerID = &id
	}

	// make sure the model is initialized
	Init(s.authenticator.policy.AccessTokenModel)

	// prepare access token
	accessToken := Init(newStructPointer(s.authenticator.policy.AccessTokenModel).(Model))

	// create access token
	accessToken.Set("Signature", signature)
	accessToken.Set("RequestedAt", request.GetRequestedAt())
	accessToken.Set("GrantedScopes", request.GetGrantedScopes())
	accessToken.Set("ClientID", &clientID)
	accessToken.Set("OwnerID", ownerID)

	// save access token
	return s.authenticator.db.C(accessToken.Meta().Collection).Insert(accessToken)
}

func (s *authenticatorStorage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	// make sure the model is initialized
	Init(s.authenticator.policy.AccessTokenModel)

	// prepare object
	obj := newStructPointer(s.authenticator.policy.AccessTokenModel)

	// fetch access token
	err := s.authenticator.db.C(s.authenticator.policy.AccessTokenModel.Meta().Collection).Find(bson.M{
		"signature": signature,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrAccessDenied
	} else if err != nil {
		return nil, err
	}

	// initialize access token
	accessToken := Init(obj.(Model))

	// create request
	req := fosite.NewRequest()
	req.RequestedAt = accessToken.Get("RequestedAt").(time.Time)
	req.GrantedScopes = accessToken.Get("GrantedScopes").([]string)
	req.Session = session

	// assign access token to context
	ctx.(*gin.Context).Set("fire.access_token", accessToken)

	return req, nil
}

func (s *authenticatorStorage) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	return errors.New("not implemented")
}

func (s *authenticatorStorage) CreateRefreshTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return errors.New("not implemented")
}

func (s *authenticatorStorage) GetRefreshTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	return nil, errors.New("not implemented")
}

func (s *authenticatorStorage) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	return errors.New("not implemented")
}

func (s *authenticatorStorage) Authenticate(ctx context.Context, id string, secret string) error {
	var model Model

	// get owner from context
	model = ctx.Value("owner").(Model)

	// get secret field
	ownerSecretField := s.authenticator.policy.OwnerModel.Meta().FieldWithTag("verifiable")

	// check secret
	err := s.authenticator.policy.CompareStrategy(model.Get(ownerSecretField.Name).([]byte), []byte(secret))
	if err != nil {
		return fosite.ErrNotFound
	}

	return nil
}

func (s *authenticatorStorage) getOwner(id string) (Model, error) {
	// prepare object
	obj := newStructPointer(s.authenticator.policy.OwnerModel)

	// get id field
	ownerIDField := s.authenticator.policy.OwnerModel.Meta().FieldWithTag("identifiable")

	// query db
	err := s.authenticator.db.C(s.authenticator.policy.OwnerModel.Meta().Collection).Find(bson.M{
		ownerIDField.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidRequest
	} else if err != nil {
		return nil, err
	}

	// initialize model
	return Init(obj.(Model)), nil
}
