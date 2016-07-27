package fire

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kr/pretty"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/handler/core"
	"github.com/ory-am/fosite/handler/core/owner"
	"github.com/ory-am/fosite/handler/core/strategy"
	"github.com/ory-am/fosite/token/hmac"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type AccessToken struct {
	Base         `bson:",inline" fire:"access-token:access-tokens:access_tokens"`
	Signature    string                `json:"-" valid:"-"`
	PlainRequest *fosite.Request       `json:"-" valid:"-" bson:"plain_request"`
	PlainClient  *fosite.DefaultClient `json:"-" valid:"-" bson:"plain_client"`
}

type Authenticator struct {
	storage *storage

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
	s := &storage{
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
	// this handler is responsible for the resource owner password credentials grant
	ownerHandler := &owner.ResourceOwnerPasswordCredentialsGrantHandler{
		HandleHelper:                                 a.handleHelper,
		ResourceOwnerPasswordCredentialsGrantStorage: a.storage,
	}

	// add handler to fosite
	a.fosite.TokenEndpointHandlers.Append(ownerHandler)

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
}

// EnableCredentialsGrant enables the usage of the OAuth 2.0 Client Credentials Grant.
func (a *Authenticator) EnableCredentialsGrant() {

}

// EnableImplicitGrant enables the usage of the OAuth 2.0 Implicit Grant.
func (a *Authenticator) EnableImplicitGrant() {

}

// HashPassword returns an Authenticator compatible hash of the password.
func (a *Authenticator) HashPassword(password string) ([]byte, error) {
	return a.fosite.Hasher.Hash([]byte(password))
}

func (a *Authenticator) Register(prefix string, router gin.IRouter) {
	router.POST(prefix+"/token", a.tokenEndpoint)
	//http.HandleFunc("/auth", authEndpoint)
}

func (a *Authenticator) Authorizer() Callback {
	return func(ctx *Context) (error, error) {
		// prepare fosite
		fctx := fosite.NewContext()
		session := &strategy.HMACSession{}

		// validate request
		_, err := a.fosite.ValidateRequestAuthorization(fctx, ctx.GinContext.Request, session, "fire")
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
	s := make(map[string]interface{})

	// obtain access request
	req, err := a.fosite.NewAccessRequest(f, ctx.Request, s)
	if err != nil {
		pretty.Println(err)
		a.fosite.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// grant the mandatory scope
	req.GrantScope("fire")

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

type storage struct {
	db               *mgo.Database
	ownerModel       Model
	ownerIDAttr      attribute
	ownerSecretAttr  attribute
	clientModel      Model
	clientIDAttr     attribute
	clientSecretAttr attribute
}

func (s *storage) GetClient(id string) (fosite.Client, error) {
	// prepare object
	obj := newStructPointer(s.clientModel)

	// query db
	err := s.db.C(s.clientModel.Collection()).Find(bson.M{
		s.clientIDAttr.dbField: id,
	}).One(obj)
	if err != nil {
		return nil, err
	}

	// initialize model
	_client := Init(obj.(Model))

	// TODO: We shouldn't use Attribute() as the field might be hidden.

	return &fosite.DefaultClient{
		ID:            id,
		Secret:        _client.Attribute(s.clientSecretAttr.name).([]byte),
		GrantTypes:    []string{"password"},
		ResponseTypes: []string{"token"},
	}, nil
}

func (s *storage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	req := fosite.NewRequest()
	req.Merge(request)

	client := req.Client.(*fosite.DefaultClient)
	req.Client = nil

	accessToken := Init(&AccessToken{
		Signature:    signature,
		PlainRequest: req,
		PlainClient:  client,
	})

	return s.db.C(accessTokenModel.Collection()).Insert(accessToken)
}

func (s *storage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	accessToken := AccessToken{}
	accessToken.PlainRequest = fosite.NewRequest()
	accessToken.PlainClient = &fosite.DefaultClient{}

	err := s.db.C(accessTokenModel.Collection()).Find(bson.M{
		"signature": signature,
	}).One(&accessToken)
	if err != nil {
		return nil, err
	}

	accessToken.PlainRequest.Client = accessToken.PlainClient
	accessToken.PlainRequest.Session = session

	return accessToken.PlainRequest, nil
}

func (s *storage) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	pretty.Println("DeleteAccessTokenSession", ctx, signature)
	return nil
}

func (s *storage) CreateRefreshTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	pretty.Println("CreateRefreshTokenSession", ctx, signature, request)
	return nil
}

func (s *storage) GetRefreshTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	pretty.Println("GetRefreshTokenSession", ctx, signature, session)
	return nil, errors.New("error get refresh token session")
}

func (s *storage) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	pretty.Println("DeleteRefreshTokenSession", ctx, signature)
	return nil
}

func (s *storage) PersistRefreshTokenGrantSession(ctx context.Context, requestRefreshSignature, accessSignature, refreshSignature string, request fosite.Requester) error {
	pretty.Println("PersistRefreshTokenGrantSession", ctx, requestRefreshSignature, accessSignature, refreshSignature, request)
	return nil
}

func (s *storage) Authenticate(ctx context.Context, id string, secret string) error {
	// prepare object
	obj := newStructPointer(s.ownerModel)

	// query db
	err := s.db.C(s.ownerModel.Collection()).Find(bson.M{
		s.ownerIDAttr.dbField: id,
	}).One(obj)
	if err != nil {
		return err
	}

	// initialize model
	owner := Init(obj.(Model))

	// check secret
	return bcrypt.CompareHashAndPassword(owner.Attribute(s.ownerSecretAttr.name).([]byte), []byte(secret))
}
