package fire

import (
	"github.com/ory-am/fosite"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/net/context"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type authenticatorStorage struct {
	db                 *mgo.Database
	ownerModel         Model
	ownerIDAttr        attribute
	ownerSecretAttr    attribute
	clientModel        Model
	clientIDAttr       attribute
	clientSecretAttr   attribute
	clientCallableAttr attribute
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
	if err != nil {
		return nil, err
	}

	// initialize model
	_client := Init(obj.(Model))

	// TODO: We shouldn't use Attribute() as the field might be hidden.

	return &authenticatorClient{
		DefaultClient: fosite.DefaultClient{
			ID:            id,
			Secret:        _client.Attribute(s.clientSecretAttr.jsonName).([]byte),
			GrantTypes:    []string{"password", "client_credentials", "implicit"},
			ResponseTypes: []string{"token"},
			RedirectURIs:  []string{_client.Attribute(s.clientCallableAttr.jsonName).(string)},
		},
		model: _client,
	}, nil
}

func (s *authenticatorStorage) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	// create access token
	accessToken := Init(&AccessToken{
		Signature:     signature,
		RequestedAt:   request.GetRequestedAt(),
		GrantedScopes: request.GetGrantedScopes(),
	})

	// TODO: Save Client Id.

	// save access token
	return s.db.C(accessTokenModel.Collection()).Insert(accessToken)
}

func (s *authenticatorStorage) GetAccessTokenSession(ctx context.Context, signature string, session interface{}) (fosite.Requester, error) {
	// fetch access token
	var accessToken AccessToken
	err := s.db.C(accessTokenModel.Collection()).Find(bson.M{
		"signature": signature,
	}).One(&accessToken)
	if err != nil {
		return nil, err
	}

	// create request
	req := fosite.NewRequest()
	req.RequestedAt = accessToken.RequestedAt
	req.GrantedScopes = accessToken.GrantedScopes
	req.Session = session

	return req, nil
}

func (s *authenticatorStorage) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	// TODO: Currently not implemented in fosite?
	return nil
}

func (s *authenticatorStorage) Authenticate(ctx context.Context, id string, secret string) error {
	var model Model

	// get owner from context
	model = ctx.Value("owner").(Model)

	// check secret
	err := bcrypt.CompareHashAndPassword(model.Attribute(s.ownerSecretAttr.jsonName).([]byte), []byte(secret))
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
	if err != nil {
		return nil, err
	}

	// initialize model
	return Init(obj.(Model)), nil
}
