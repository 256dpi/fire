package oauth2

import (
	"time"

	"github.com/gonfire/fire/model"
	"gopkg.in/mgo.v2/bson"
)

// AccessTokenModel is the interface that must be implemented to provide a
// custom access token model.
type AccessTokenModel interface {
	model.Model

	OAuthIdentifier() string
	GetOAuthData() (requestedAt time.Time, grantedScopes []string)
	SetOAuthData(signature string, grantedScopes []string, clientID bson.ObjectId, ownerID *bson.ObjectId)
}

// AccessToken is the built-in model used to store access tokens. The model
// can be mounted using a controller to become manageable an API.
type AccessToken struct {
	model.Base    `json:"-" bson:",inline" fire:"access-tokens:access_tokens"`
	Signature     string         `json:"signature" valid:"required"`
	RequestedAt   time.Time      `json:"requested-at" valid:"required" bson:"requested_at"`
	GrantedScopes []string       `json:"granted-scopes" valid:"required" bson:"granted_scopes"`
	ClientID      bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id"`
	OwnerID       *bson.ObjectId `json:"owner-id" valid:"-" bson:"owner_id"`
}

// OAuthIdentifier implements the AccessTokenModel interface.
func (t *AccessToken) OAuthIdentifier() string {
	return "Signature"
}

// GetOAuthData implements the AccessTokenModel interface.
func (t *AccessToken) GetOAuthData() (time.Time, []string) {
	return t.RequestedAt, t.GrantedScopes
}

// SetOAuthData implements the AccessTokenModel interface.
func (t *AccessToken) SetOAuthData(signature string, grantedScopes []string, clientID bson.ObjectId, ownerID *bson.ObjectId) {
	t.RequestedAt = time.Now()
	t.Signature = signature
	t.GrantedScopes = grantedScopes
	t.ClientID = clientID
	t.OwnerID = ownerID
}

// ClientModel is the interface that must be implemented to provide a custom
// client model.
type ClientModel interface {
	model.Model

	OAuthIdentifier() string
	GetOAuthData() (secretHash []byte, scopes []string, grantTypes []string, callbacks []string)
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

// OAuthIdentifier implements the ClientModel interface.
func (a *Application) OAuthIdentifier() string {
	return "Key"
}

// GetOAuthData implements the ClientModel interface.
func (a *Application) GetOAuthData() ([]byte, []string, []string, []string) {
	return a.SecretHash, a.Scopes, a.GrantTypes, a.Callbacks
}

// OwnerModel is the interface that must be implemented to provide a custom
// owner model.
type OwnerModel interface {
	model.Model

	OAuthIdentifier() string
	GetOAuthData() (passwordHash []byte)
}

// User is the built-in model used to store users. The model can be mounted as a
// fire Resource to become manageable via the API.
type User struct {
	model.Base   `json:"-" bson:",inline" fire:"users"`
	Name         string `json:"name" valid:"required"`
	Email        string `json:"email" valid:"required"`
	PasswordHash []byte `json:"-" valid:"required"`
}

// OAuthIdentifier implements the OwnerModel interface.
func (u *User) OAuthIdentifier() string {
	return "Email"
}

// GetOAuthData implements the OwnerModel interface.
func (u *User) GetOAuthData() []byte {
	return u.PasswordHash
}
