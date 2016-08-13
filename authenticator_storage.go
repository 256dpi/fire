package fire

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/ory-am/fosite"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type authenticatorStorage struct {
	authenticator *Authenticator

	db                  *mgo.Database
	ownerModel          Model
	ownerIDAttr         attribute
	ownerSecretAttr     attribute
	clientModel         Model
	clientIDAttr        attribute
	clientSecretAttr    attribute
	clientGrantableAttr attribute
	clientCallableAttr  attribute
}

type authenticatorClient struct {
	fosite.DefaultClient
	model Model
}

func (s *authenticatorStorage) GetClient(id string) (fosite.Client, error) {
	// prepare object
	obj := newStructPointer(s.clientModel)

	// query db
	err := s.db.C(s.clientModel.Collection()).Find(bson.M{
		s.clientIDAttr.bsonName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidClient
	} else if err != nil {
		return nil, err
	}

	// initialize model
	_client := Init(obj.(Model))

	// TODO: Calculate GrantTypes based on enabled grants.

	return &authenticatorClient{
		DefaultClient: fosite.DefaultClient{
			ID:            id,
			Secret:        _client.Attribute(s.clientSecretAttr.fieldName).([]byte),
			GrantTypes:    []string{"password", "client_credentials", "implicit"},
			ResponseTypes: []string{"token"},
			RedirectURIs:  _client.Attribute(s.clientCallableAttr.fieldName).([]string),
			Scopes:        _client.Attribute(s.clientGrantableAttr.fieldName).([]string),
		},
		model: _client,
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
	// retrieve optional owner id
	var ownerID *bson.ObjectId
	if ctx.Value("owner") != nil {
		id := ctx.Value("owner").(Model).ID()
		ownerID = &id
	}

	// create access token
	accessToken := Init(&AccessToken{
		Signature:     signature,
		RequestedAt:   request.GetRequestedAt(),
		GrantedScopes: request.GetGrantedScopes(),
		ClientID:      ctx.Value("client").(Model).ID(),
		OwnerID:       ownerID,
	})

	// save access token
	return s.db.C(accessTokenModel.Collection()).Insert(accessToken)
}

func (s *authenticatorStorage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	// fetch access token
	var accessToken AccessToken
	err := s.db.C(accessTokenModel.Collection()).Find(bson.M{
		"signature": signature,
	}).One(&accessToken)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrAccessDenied
	} else if err != nil {
		return nil, err
	}

	// create request
	req := fosite.NewRequest()
	req.RequestedAt = accessToken.RequestedAt
	req.GrantedScopes = accessToken.GrantedScopes
	req.Session = session

	// assign access token to context
	ctx.(*gin.Context).Set("fire.access_token", &accessToken)

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

	// check secret
	err := bcrypt.CompareHashAndPassword(model.Attribute(s.ownerSecretAttr.fieldName).([]byte), []byte(secret))
	if err != nil {
		return fosite.ErrNotFound
	}

	return nil
}

func (s *authenticatorStorage) getOwner(id string) (Model, error) {
	// prepare object
	obj := newStructPointer(s.ownerModel)

	// query db
	err := s.db.C(s.ownerModel.Collection()).Find(bson.M{
		s.ownerIDAttr.bsonName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, fosite.ErrInvalidRequest
	} else if err != nil {
		return nil, err
	}

	// initialize model
	return Init(obj.(Model)), nil
}
