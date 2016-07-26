package fire

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kr/pretty"
	"github.com/ory-am/fosite"
	"github.com/ory-am/fosite/handler/core"
	"github.com/ory-am/fosite/handler/core/owner"
	"github.com/ory-am/fosite/handler/core/refresh"
	"github.com/ory-am/fosite/handler/core/strategy"
	"github.com/ory-am/fosite/token/hmac"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Authenticator struct {
	backend *fosite.Fosite
}

func NewAuthenticator(db *mgo.Database, model Model, collection, secret string) *Authenticator {
	model = Init(model)

	// prepare lists
	var identifiable []attribute
	var verifiable []attribute

	// find identifiable and verifiable attributes
	for _, attr := range model.getBase().attributes {
		if attr.identifiable {
			identifiable = append(identifiable, attr)
		}

		if attr.verifiable {
			verifiable = append(verifiable, attr)
		}
	}

	// check lists
	if len(identifiable) != 1 || len(verifiable) != 1 {
		panic("expected to find one identifiable and one verifiable attribute")
	}

	s := &storage{
		db:           db,
		model:        model,
		collection:   collection,
		identifiable: identifiable[0],
		verifiable:   verifiable[0],
	}

	// create a new token generation strategy
	strategy := &strategy.HMACSHAStrategy{
		Enigma: &hmac.HMACStrategy{
			GlobalSecret: []byte(secret),
		},
		AccessTokenLifespan:   time.Hour,
		AuthorizeCodeLifespan: time.Hour,
	}

	// instantiate a new fosite instance
	f := fosite.NewFosite(s)

	// set mandatory scope
	f.MandatoryScope = "fire"

	// set the default access token lifespan to one hour
	accessTokenLifespan := time.Hour

	// this little helper is used by some of the handlers below
	oauth2HandleHelper := &core.HandleHelper{
		AccessTokenStrategy: strategy,
		AccessTokenStorage:  s,
		AccessTokenLifespan: accessTokenLifespan,
	}

	// this handler is responsible for the resource owner password credentials grant
	ownerHandler := &owner.ResourceOwnerPasswordCredentialsGrantHandler{
		HandleHelper:                                 oauth2HandleHelper,
		ResourceOwnerPasswordCredentialsGrantStorage: s,
	}

	// add handler to fosite
	f.TokenEndpointHandlers.Append(ownerHandler)

	// this handler is responsible for the refresh token grant
	refreshHandler := &refresh.RefreshTokenGrantHandler{
		AccessTokenStrategy:      strategy,
		RefreshTokenStrategy:     strategy,
		RefreshTokenGrantStorage: s,
		AccessTokenLifespan:      accessTokenLifespan,
	}

	// add handler to fosite
	f.TokenEndpointHandlers.Append(refreshHandler)

	// add a request validator for access tokens to fosite
	f.AuthorizedRequestValidators.Append(&core.CoreValidator{
		AccessTokenStrategy: strategy,
		AccessTokenStorage:  s,
	})

	return &Authenticator{
		backend: f,
	}
}

func (a *Authenticator) HashPassword(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), 0)
}

func (a *Authenticator) Register(prefix string, router gin.IRouter) {
	router.POST(prefix+"/token", a.tokenEndpoint)
	//http.HandleFunc("/auth", authEndpoint)
}

func (a *Authenticator) tokenEndpoint(ctx *gin.Context) {
	// create new context
	f := fosite.NewContext()

	// create new session
	s := make(map[string]interface{})

	// obtain access request
	req, err := a.backend.NewAccessRequest(f, ctx.Request, s)
	if err != nil {
		println(err.Error())
		a.backend.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// grant the mandatory scope
	req.GrantScope("fire")

	// obtain access response
	res, err := a.backend.NewAccessResponse(f, ctx.Request, req)
	if err != nil {
		println(err.Error())
		a.backend.WriteAccessError(ctx.Writer, req, err)
		return
	}

	// write response
	a.backend.WriteAccessResponse(ctx.Writer, req, res)
}

type client struct {
	id     string
	secret []byte
}

func (c *client) Grant(requestScope string) bool {
	return false
}

func (c *client) GetID() string {
	return c.id
}

func (c *client) GetRedirectURIs() []string {
	return nil
}

func (c *client) GetHashedSecret() []byte {
	return c.secret
}

func (c *client) GetGrantedScopes() fosite.Scopes {
	return c
}

func (c *client) GetGrantTypes() fosite.Arguments {
	return fosite.Arguments{"password", "refresh_token"}
}

func (c *client) GetResponseTypes() fosite.Arguments {
	return fosite.Arguments{"token"}
}

func (c *client) GetOwner() string {
	return ""
}

type storage struct {
	db           *mgo.Database
	model        Model
	collection   string
	identifiable attribute
	verifiable   attribute
}

func (s *storage) GetClient(id string) (fosite.Client, error) {
	model, err := s.findClient(id)
	if err != nil {
		return nil, err
	}

	return &client{
		id:     id,
		secret: model.Attribute(s.verifiable.name).([]byte),
	}, nil
}

func (s *storage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	pretty.Println("CreateAccessTokenSession", ctx, signature, request)
	return nil
}

func (s *storage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	pretty.Println("GetAccessTokenSession", ctx, signature, session)
	return nil, errors.New("error get access token session")
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
	// get client
	client, err := s.findClient(id)
	if err != nil {
		return err
	}

	// check secret
	return bcrypt.CompareHashAndPassword(client.Attribute(s.verifiable.name).([]byte), []byte(secret))
}

func (s *storage) findClient(id string) (Model, error) {
	// prepare object
	obj := newStructPointer(s.model)

	// query db
	err := s.db.C(s.collection).Find(bson.M{
		s.identifiable.dbField: id,
	}).One(obj)
	if err != nil {
		return nil, err
	}

	// initialize and return model
	return Init(obj.(Model)), nil
}
