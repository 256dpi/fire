package auth

import (
	"time"

	"github.com/gonfire/fire/model"
	"github.com/gonfire/oauth2"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

type TokenData struct {
	Signature       string
	Scope           oauth2.Scope
	ExpiresAt       time.Time
	ClientID        bson.ObjectId
	ResourceOwnerID *bson.ObjectId
}

// Token is the interface that must be implemented to provide a custom
// access token and refresh token model.
type Token interface {
	model.Model

	TokenIdentifier() string
	GetTokenData() *TokenData
	SetTokenData(*TokenData)
}

// TODO: We need to separate the models otherwise a refresh token can be used to
// access data.

// Credential is the built-in model used to store tokens.
type Credential struct {
	model.Base      `json:"-" bson:",inline" fire:"credentials"`
	Signature       string         `json:"signature" valid:"required"`
	ExpiresAt       time.Time      `json:"expires-at" valid:"required" bson:"expires_at"`
	Scope           string         `json:"scope" valid:"required" bson:"scope"`
	ClientID        bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id"`
	ResourceOwnerID *bson.ObjectId `json:"resource-owner-id" valid:"-" bson:"resource_owner_id"`
}

// TokenIdentifier implements the Token interface.
func (c *Credential) TokenIdentifier() string {
	return "Signature"
}

// GetTokenData implements the Token interface.
func (c *Credential) GetTokenData() *TokenData {
	return &TokenData{
		Signature:       c.Signature,
		Scope:           oauth2.ParseScope(c.Scope),
		ExpiresAt:       c.ExpiresAt,
		ClientID:        c.ClientID,
		ResourceOwnerID: c.ResourceOwnerID,
	}
}

// SetTokenData implements the Token interface.
func (c *Credential) SetTokenData(data *TokenData) {
	c.Signature = data.Signature
	c.Scope = data.Scope.String()
	c.ExpiresAt = data.ExpiresAt
	c.ClientID = data.ClientID
	c.ResourceOwnerID = data.ResourceOwnerID
}

// Client is the interface that must be implemented to provide a custom client
// model.
type Client interface {
	model.Model

	ClientIdentifier() string
	ValidRedirectURI(string) bool
	ValidSecret(string) bool
}

// Application is the built-in model used to store clients.
type Application struct {
	model.Base   `json:"-" bson:",inline" fire:"applications"`
	Name         string   `json:"name" valid:"required"`
	Key          string   `json:"key" valid:"required"`
	SecretHash   []byte   `json:"-" valid:"required"`
	Scope        string   `json:"scope" valid:"required"`
	RedirectURIs []string `json:"redirect_uris" valid:"required"`
}

// ClientIdentifier implements the Client interface.
func (a *Application) ClientIdentifier() string {
	return "Key"
}

// ValidRedirectURI implements the Client interface.
func (a *Application) ValidRedirectURI(uri string) bool {
	for _, r := range a.RedirectURIs {
		if r == uri {
			return true
		}
	}

	return false
}

// ValidSecret implements the Client interface.
func (a *Application) ValidSecret(secret string) bool {
	return bcrypt.CompareHashAndPassword(a.SecretHash, []byte(secret)) == nil
}

// ResourceOwner is the interface that must be implemented to provide a custom
// resource owner model.
type ResourceOwner interface {
	model.Model

	ResourceOwnerIdentifier() string
	ValidPassword(string) bool
}

// User is the built-in model used to store resource owners.
type User struct {
	model.Base   `json:"-" bson:",inline" fire:"users"`
	Name         string `json:"name" valid:"required"`
	Email        string `json:"email" valid:"required"`
	PasswordHash []byte `json:"-" valid:"required"`
}

// ResourceOwnerIdentifier implements the ResourceOwner interface.
func (u *User) ResourceOwnerIdentifier() string {
	return "Email"
}

// ValidPassword implements the ResourceOwner interface.
func (u *User) ValidPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)) == nil
}
