package oauth2

import (
	"errors"
	"reflect"

	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/ory-am/fosite"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var typeOfIdentifier = reflect.TypeOf(Identifier(""))

type abstractClient struct {
	fosite.DefaultClient
	model ClientModel
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

	// get id field
	field := s.getIdentifierField(s.authenticator.policy.ClientModel)

	// query db
	err := store.C(s.authenticator.policy.ClientModel).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidClient
	} else if err != nil {
		return nil, err
	}

	// initialize model
	client := model.Init(obj).(ClientModel)

	// extract client data
	secretHash, scopes, grantTypes, callbacks := client.GetOAuthData()

	return &abstractClient{
		DefaultClient: fosite.DefaultClient{
			ID:            id,
			Secret:        secretHash,
			GrantTypes:    grantTypes,
			ResponseTypes: []string{"token"},
			RedirectURIs:  callbacks,
			Scopes:        scopes,
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
	clientID := ctx.Value("client").(ClientModel).ID()

	// retrieve optional owner id
	var ownerID *bson.ObjectId
	if ctx.Value("owner") != nil {
		id := ctx.Value("owner").(OwnerModel).ID()
		ownerID = &id
	}

	// prepare scopes
	scopes := []string(request.GetGrantedScopes())

	// make sure the model is initialized
	model.Init(s.authenticator.policy.AccessTokenModel)

	// prepare access token
	accessToken := model.Init(s.authenticator.policy.AccessTokenModel.Meta().Make())

	// set access token data
	accessToken.(AccessTokenModel).SetOAuthData(signature, scopes, clientID, ownerID)

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

	// get signature field
	field := s.getIdentifierField(s.authenticator.policy.AccessTokenModel)

	// fetch access token
	err := store.C(s.authenticator.policy.AccessTokenModel).Find(bson.M{
		field.BSONName: signature,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrAccessDenied
	} else if err != nil {
		return nil, err
	}

	// initialize access token
	accessToken := model.Init(obj).(AccessTokenModel)

	// get access token data
	requestedAt, grantedScopes := accessToken.GetOAuthData()

	// create request
	req := fosite.NewRequest()
	req.RequestedAt = requestedAt
	req.GrantedScopes = grantedScopes
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

	// extract data from owner
	passwordHash := owner.GetOAuthData()

	// check secret
	err = s.authenticator.policy.CompareStrategy(passwordHash, []byte(secret))
	if err != nil {
		return fosite.ErrNotFound
	}

	return nil
}

func (s *storage) getIdentifierField(m model.Model) model.Field {
	for _, field := range m.Meta().Fields {
		if field.Type == typeOfIdentifier {
			return field
		}
	}

	panic("Missing Identifier field for " + m.Meta().Name)
}

func (s *storage) getOwner(id string) (OwnerModel, error) {
	// prepare object
	obj := s.authenticator.policy.OwnerModel.Meta().Make()

	// get store
	store := s.authenticator.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get id field
	field := s.getIdentifierField(s.authenticator.policy.OwnerModel)

	// query db
	err := store.C(s.authenticator.policy.OwnerModel).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidRequest
	} else if err != nil {
		return nil, err
	}

	// initialize model
	return model.Init(obj).(OwnerModel), nil
}
