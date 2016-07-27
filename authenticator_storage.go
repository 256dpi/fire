package fire

import (
	"errors"

	"github.com/kr/pretty"
	"github.com/ory-am/fosite"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type authenticatorStorage struct {
	db               *mgo.Database
	ownerModel       Model
	ownerIDAttr      attribute
	ownerSecretAttr  attribute
	clientModel      Model
	clientIDAttr     attribute
	clientSecretAttr attribute
}

func (s *authenticatorStorage) GetClient(id string) (fosite.Client, error) {
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

func (s *authenticatorStorage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
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

func (s *authenticatorStorage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
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

func (s *authenticatorStorage) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	pretty.Println("DeleteAccessTokenSession", ctx, signature)
	return nil
}

func (s *authenticatorStorage) CreateRefreshTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	pretty.Println("CreateRefreshTokenSession", ctx, signature, request)
	return nil
}

func (s *authenticatorStorage) GetRefreshTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	pretty.Println("GetRefreshTokenSession", ctx, signature, session)
	return nil, errors.New("error get refresh token session")
}

func (s *authenticatorStorage) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	pretty.Println("DeleteRefreshTokenSession", ctx, signature)
	return nil
}

func (s *authenticatorStorage) PersistRefreshTokenGrantSession(ctx context.Context, requestRefreshSignature, accessSignature, refreshSignature string, request fosite.Requester) error {
	pretty.Println("PersistRefreshTokenGrantSession", ctx, requestRefreshSignature, accessSignature, refreshSignature, request)
	return nil
}

func (s *authenticatorStorage) Authenticate(ctx context.Context, id string, secret string) error {
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
