package oauth2

import (
	"errors"
	"time"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/ory-am/fosite"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type abstractClient struct {
	fosite.DefaultClient
	model model.Model
}

type storage struct {
	authenticator *Authenticator
}

func (s *storage) GetClient(id string) (fosite.Client, error) {
	// prepare object
	obj := s.authenticator.policy.ClientModel.Meta().Make()

	// get store
	store := s.authenticator.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// query db
	err := store.C(s.authenticator.policy.ClientModel).Find(bson.M{
		s.authenticator.policy.ClientIDField: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidClient
	} else if err != nil {
		return nil, err
	}

	// initialize model
	client := model.Init(obj.(model.Model))

	// extract client data
	clientData := s.authenticator.policy.ClientExtractor(client)

	return &abstractClient{
		DefaultClient: fosite.DefaultClient{
			ID:            id,
			Secret:        clientData["SecretHash"].([]byte),
			GrantTypes:    clientData["GrantTypes"].([]string),
			ResponseTypes: []string{"token"},
			RedirectURIs:  clientData["Callbacks"].([]string),
			Scopes:        clientData["Scopes"].([]string),
		},
		model: client,
	}, nil
}

func (s *storage) CreateAuthorizeCodeSession(ctx context.Context, code string, request fosite.Requester) error {
	return errors.New("not implemented")
}

func (s *storage) GetAuthorizeCodeSession(ctx context.Context, code string, session interface{}) (fosite.Requester, error) {
	return nil, errors.New("not implemented")
}

func (s *storage) DeleteAuthorizeCodeSession(ctx context.Context, code string) error {
	return errors.New("not implemented")
}

func (s *storage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	// retrieve client id
	clientID := ctx.Value("client").(model.Model).ID()

	// retrieve optional owner id
	var ownerID *bson.ObjectId
	if ctx.Value("owner") != nil {
		id := ctx.Value("owner").(model.Model).ID()
		ownerID = &id
	}

	// make sure the model is initialized
	model.Init(s.authenticator.policy.AccessTokenModel)

	// prepare access token
	accessToken := model.Init(s.authenticator.policy.AccessTokenModel.Meta().Make())

	// inject data
	s.authenticator.policy.AccessTokenInjector(accessToken, fire.Map{
		"Signature":     signature,
		"RequestedAt":   request.GetRequestedAt(),
		"GrantedScopes": []string(request.GetGrantedScopes()),
		"ClientID":      clientID,
		"OwnerID":       ownerID,
	})

	// get store
	store := s.authenticator.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// save access token
	return store.C(accessToken).Insert(accessToken)
}

func (s *storage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	// prepare object
	obj := s.authenticator.policy.AccessTokenModel.Meta().Make()

	// get store
	store := s.authenticator.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// fetch access token
	err := store.C(s.authenticator.policy.AccessTokenModel).Find(bson.M{
		s.authenticator.policy.AccessTokenIDField: signature,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrAccessDenied
	} else if err != nil {
		return nil, err
	}

	// initialize access token
	accessToken := model.Init(obj.(model.Model))

	// extract access token data
	accessTokenData := s.authenticator.policy.AccessTokenExtractor(accessToken)

	// create request
	req := fosite.NewRequest()
	req.RequestedAt = accessTokenData["RequestedAt"].(time.Time)
	req.GrantedScopes = accessTokenData["GrantedScopes"].([]string)
	req.Session = session

	// assign access token to context
	ctx.(echo.Context).Set("fire.access_token", accessToken)

	return req, nil
}

func (s *storage) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	return errors.New("not implemented")
}

func (s *storage) CreateRefreshTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return errors.New("not implemented")
}

func (s *storage) GetRefreshTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	return nil, errors.New("not implemented")
}

func (s *storage) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	return errors.New("not implemented")
}

func (s *storage) Authenticate(ctx context.Context, id string, secret string) error {
	// retrieve owner
	owner, err := s.getOwner(id)
	if err != nil {
		return fosite.ErrAccessDenied
	}

	// assign owner to context
	ctx.(echo.Context).Set("owner", owner)

	// get owner from context
	owner = ctx.Value("owner").(model.Model)

	// extract data from owner
	ownerData := s.authenticator.policy.OwnerExtractor(owner)

	// check secret
	err = s.authenticator.policy.CompareStrategy(ownerData["PasswordHash"].([]byte), []byte(secret))
	if err != nil {
		return fosite.ErrNotFound
	}

	return nil
}

func (s *storage) getOwner(id string) (model.Model, error) {
	// prepare object
	obj := s.authenticator.policy.OwnerModel.Meta().Make()

	// get store
	store := s.authenticator.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// query db
	err := store.C(s.authenticator.policy.OwnerModel).Find(bson.M{
		s.authenticator.policy.OwnerIDField: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidRequest
	} else if err != nil {
		return nil, err
	}

	// initialize model
	return model.Init(obj.(model.Model)), nil
}
