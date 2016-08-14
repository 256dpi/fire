package fire

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ory-am/fosite"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type authenticatorStorage struct {
	authenticator *Authenticator

	db               *mgo.Database
	ownerModel       Model
	clientModel      Model
	accessTokenModel Model
}

type authenticatorClient struct {
	fosite.DefaultClient
	model Model
}

func (s *authenticatorStorage) GetClient(id string) (fosite.Client, error) {
	// prepare object
	obj := newStructPointer(s.clientModel)

	// read fields
	clientIDField := s.clientModel.FieldWithTag("identifiable")
	clientSecretField := s.clientModel.FieldWithTag("verifiable")
	clientCallableField := s.clientModel.FieldWithTag("callable")
	clientGrantableField := s.clientModel.FieldWithTag("grantable")

	// query db
	err := s.db.C(s.clientModel.Collection()).Find(bson.M{
		clientIDField.BSONName: id,
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
			Secret:        client.Get(clientSecretField.Name).([]byte),
			GrantTypes:    s.authenticator.enabledGrants,
			ResponseTypes: []string{"token"},
			RedirectURIs:  client.Get(clientCallableField.Name).([]string),
			Scopes:        client.Get(clientGrantableField.Name).([]string),
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
	// retrieve optional owner id
	var ownerID *bson.ObjectId
	if ctx.Value("owner") != nil {
		id := ctx.Value("owner").(Model).ID()
		ownerID = &id
	}

	// prepare access token
	accessToken := Init(newStructPointer(s.accessTokenModel).(Model))

	// create access token
	accessToken.Set("Type", "access_token")
	accessToken.Set("Signature", signature)
	accessToken.Set("RequestedAt", request.GetRequestedAt())
	accessToken.Set("GrantedScopes", request.GetGrantedScopes())
	accessToken.Set("ClientID", ctx.Value("client").(Model).ID())
	accessToken.Set("OwnerID", ownerID)

	// save access token
	return s.db.C(accessToken.Collection()).Insert(accessToken)
}

func (s *authenticatorStorage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	// prepare object
	obj := newStructPointer(s.accessTokenModel)

	// fetch access token
	err := s.db.C(s.accessTokenModel.Collection()).Find(bson.M{
		"type":      "access_token",
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

	// get secret field
	ownerSecretField := s.ownerModel.FieldWithTag("verifiable")

	// check secret
	err := s.authenticator.CompareCallback(model.Get(ownerSecretField.Name).([]byte), []byte(secret))
	if err != nil {
		return fosite.ErrNotFound
	}

	return nil
}

func (s *authenticatorStorage) getOwner(id string) (Model, error) {
	// prepare object
	obj := newStructPointer(s.ownerModel)

	// get id field
	ownerIDField := s.ownerModel.FieldWithTag("identifiable")

	// query db
	err := s.db.C(s.ownerModel.Collection()).Find(bson.M{
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
