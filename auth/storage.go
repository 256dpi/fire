package auth

import (
	"github.com/gonfire/fire/model"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Storage struct {
	policy *Policy
	store  *model.Store
}

func NewStorage(policy *Policy, store *model.Store) *Storage {
	return &Storage{
		policy: policy,
		store:  store,
	}
}

func (s *Storage) GetClient(id string) (Client, error) {
	// prepare object
	obj := s.policy.Client.Meta().Make()

	// get store
	store := s.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get id field
	field := s.policy.Client.Meta().FindField(s.policy.Client.ClientIdentifier())

	// query db
	err := store.C(s.policy.Client).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// initialize model
	client := model.Init(obj).(Client)

	return client, nil
}

func (s *Storage) GetResourceOwner(id string) (ResourceOwner, error) {
	// prepare object
	obj := s.policy.ResourceOwner.Meta().Make()

	// get store
	store := s.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get id field
	field := s.policy.ResourceOwner.Meta().FindField(s.policy.ResourceOwner.ResourceOwnerIdentifier())

	// query db
	err := store.C(s.policy.ResourceOwner).Find(bson.M{
		field.BSONName: id,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// initialize model
	resourceOwner := model.Init(obj).(ResourceOwner)

	return resourceOwner, nil
}

func (s *Storage) GetAccessToken(signature string) (Token, error) {
	return s.getToken(s.policy.AccessToken, signature)
}

func (s *Storage) GetRefreshToken(signature string) (Token, error) {
	return s.getToken(s.policy.RefreshToken, signature)
}

func (s *Storage) getToken(tokenModel Token, signature string) (Token, error) {
	// prepare object
	obj := tokenModel.Meta().Make()

	// get store
	store := s.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get signature field
	field := tokenModel.Meta().FindField(tokenModel.TokenIdentifier())

	// fetch access token
	err := store.C(tokenModel).Find(bson.M{
		field.BSONName: signature,
	}).One(obj)
	if err == mgo.ErrNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	// initialize access token
	accessToken := model.Init(obj).(Token)

	return accessToken, nil
}

func (s *Storage) SaveAccessToken(data *TokenData) (Token, error) {
	return s.saveToken(s.policy.AccessToken, data)
}

func (s *Storage) SaveRefreshToken(data *TokenData) (Token, error) {
	return s.saveToken(s.policy.RefreshToken, data)
}

func (s *Storage) saveToken(tokenModel Token, data *TokenData) (Token, error) {
	// prepare access token
	token := tokenModel.Meta().Make().(Token)

	// set access token data
	token.SetTokenData(data)

	// get store
	store := s.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// save access token
	err := store.C(token).Insert(token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (s *Storage) DeleteAccessToken(signature string) error {
	return s.deleteToken(s.policy.AccessToken, signature)
}

func (s *Storage) DeleteRefreshToken(signature string) error {
	return s.deleteToken(s.policy.RefreshToken, signature)
}

func (s *Storage) deleteToken(tokenModel Token, signature string) error {
	// get store
	store := s.store.Copy()

	// ensure store gets closed
	defer store.Close()

	// get signature field
	field := tokenModel.Meta().FindField(tokenModel.TokenIdentifier())

	// fetch access token
	err := store.C(tokenModel).Remove(bson.M{
		field.BSONName: signature,
	})
	if err == mgo.ErrNotFound {
		return nil
	} else if err != nil {
		return err
	}

	return nil
}
