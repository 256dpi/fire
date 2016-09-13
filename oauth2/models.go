package oauth2

import (
	"time"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/model"
	"gopkg.in/mgo.v2/bson"
)

// AccessToken is the built-in model used to store access tokens. The model
// can be mounted using a controller to become manageable an API.
type AccessToken struct {
	model.Base    `json:"-" bson:",inline" fire:"access-tokens:access_tokens"`
	Signature     string         `json:"signature" valid:"required"`
	RequestedAt   time.Time      `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string       `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
	ClientID      bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id" fire:"filterable,sortable"`
	OwnerID       *bson.ObjectId `json:"owner-id" valid:"-" bson:"owner_id" fire:"filterable,sortable"`
}

func accessTokenExtractor(m model.Model) fire.Map {
	accessToken := m.(*AccessToken)

	return fire.Map{
		"RequestedAt":   accessToken.RequestedAt,
		"GrantedScopes": accessToken.GrantedScopes,
	}
}

func accessTokenInjector(m model.Model, data fire.Map) {
	accessToken := m.(*AccessToken)
	accessToken.Signature = data["Signature"].(string)
	accessToken.RequestedAt = data["RequestedAt"].(time.Time)
	accessToken.GrantedScopes = data["GrantedScopes"].([]string)
	accessToken.ClientID = data["ClientID"].(bson.ObjectId)
	accessToken.OwnerID = data["OwnerID"].(*bson.ObjectId)
}

// Application is the built-in model used to store clients. The model can be
// mounted as a fire Resource to become manageable via the API.
type Application struct {
	model.Base `json:"-" bson:",inline" fire:"applications"`
	Name       string   `json:"name" valid:"required"`
	Key        string   `json:"key" valid:"required"`
	SecretHash []byte   `json:"-" valid:"required"`
	Scopes     []string `json:"scopes" valid:"required"`
	GrantTypes []string `json:"grant-types" valid:"required" bson:"grant_types"`
	Callbacks  []string `json:"callbacks" valid:"required"`
}

func applicationExtractor(m model.Model) fire.Map {
	application := m.(*Application)

	return fire.Map{
		"SecretHash": application.SecretHash,
		"Scopes":     application.Scopes,
		"GrantTypes": application.GrantTypes,
		"Callbacks":  application.Callbacks,
	}
}

// User is the built-in model used to store users. The model can be mounted as a
// fire Resource to become manageable via the API.
type User struct {
	model.Base   `json:"-" bson:",inline" fire:"users"`
	Name         string `json:"name" valid:"required"`
	Email        string `json:"email" valid:"required"`
	PasswordHash []byte `json:"-" valid:"required"`
}

func userExtractor(m model.Model) fire.Map {
	return fire.Map{
		"PasswordHash": m.(*User).PasswordHash,
	}
}
