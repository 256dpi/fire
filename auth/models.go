package auth

import (
	"time"

	"github.com/gonfire/fire"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

// TokenData is used to carry token related information.
type TokenData struct {
	Signature       string
	Scope           []string
	ExpiresAt       time.Time
	ClientID        bson.ObjectId
	ResourceOwnerID *bson.ObjectId
}

// Token is the interface that must be implemented to provide a custom
// access token and refresh token fire.
type Token interface {
	fire.Model

	TokenIdentifier() string
	GetTokenData() *TokenData
	SetTokenData(*TokenData)
}

// AccessToken is the built-in model used to store access tokens.
type AccessToken struct {
	fire.Base       `json:"-" bson:",inline" fire:"access-tokens:access_tokens"`
	Signature       string         `json:"signature" valid:"required"`
	ExpiresAt       time.Time      `json:"expires-at" valid:"required" bson:"expires_at"`
	Scope           []string       `json:"scope" valid:"required" bson:"scope"`
	ClientID        bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id"`
	ResourceOwnerID *bson.ObjectId `json:"resource-owner-id" valid:"-" bson:"resource_owner_id"`
}

// TokenIdentifier implements the Token interface.
func (t *AccessToken) TokenIdentifier() string {
	return "Signature"
}

// GetTokenData implements the Token interface.
func (t *AccessToken) GetTokenData() *TokenData {
	return &TokenData{
		Signature:       t.Signature,
		Scope:           t.Scope,
		ExpiresAt:       t.ExpiresAt,
		ClientID:        t.ClientID,
		ResourceOwnerID: t.ResourceOwnerID,
	}
}

// SetTokenData implements the Token interface.
func (t *AccessToken) SetTokenData(data *TokenData) {
	t.Signature = data.Signature
	t.Scope = data.Scope
	t.ExpiresAt = data.ExpiresAt
	t.ClientID = data.ClientID
	t.ResourceOwnerID = data.ResourceOwnerID
}

// RefreshToken is the built-in model used to store refresh tokens.
type RefreshToken struct {
	fire.Base       `json:"-" bson:",inline" fire:"refresh-tokens:refresh_tokens"`
	Signature       string         `json:"signature" valid:"required"`
	ExpiresAt       time.Time      `json:"expires-at" valid:"required" bson:"expires_at"`
	Scope           []string       `json:"scope" valid:"required" bson:"scope"`
	ClientID        bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id"`
	ResourceOwnerID *bson.ObjectId `json:"resource-owner-id" valid:"-" bson:"resource_owner_id"`
}

// TokenIdentifier implements the Token interface.
func (t *RefreshToken) TokenIdentifier() string {
	return "Signature"
}

// GetTokenData implements the Token interface.
func (t *RefreshToken) GetTokenData() *TokenData {
	return &TokenData{
		Signature:       t.Signature,
		Scope:           t.Scope,
		ExpiresAt:       t.ExpiresAt,
		ClientID:        t.ClientID,
		ResourceOwnerID: t.ResourceOwnerID,
	}
}

// SetTokenData implements the Token interface.
func (t *RefreshToken) SetTokenData(data *TokenData) {
	t.Signature = data.Signature
	t.Scope = data.Scope
	t.ExpiresAt = data.ExpiresAt
	t.ClientID = data.ClientID
	t.ResourceOwnerID = data.ResourceOwnerID
}

// Client is the interface that must be implemented to provide a custom client
// fire.
type Client interface {
	fire.Model

	ClientIdentifier() string
	ValidRedirectURI(string) bool
	ValidSecret(string) bool
}

// Application is the built-in model used to store clients.
type Application struct {
	fire.Base   `json:"-" bson:",inline" fire:"applications"`
	Name        string `json:"name" valid:"required"`
	Key         string `json:"key" valid:"required"`
	SecretHash  []byte `json:"-" valid:"required"`
	Scope       string `json:"scope" valid:"required"`
	RedirectURI string `json:"redirect_uri" valid:"required"`
}

// ClientIdentifier implements the Client interface.
func (a *Application) ClientIdentifier() string {
	return "Key"
}

// ValidRedirectURI implements the Client interface.
func (a *Application) ValidRedirectURI(uri string) bool {
	return uri == a.RedirectURI
}

// ValidSecret implements the Client interface.
func (a *Application) ValidSecret(secret string) bool {
	return bcrypt.CompareHashAndPassword(a.SecretHash, []byte(secret)) == nil
}

// ResourceOwner is the interface that must be implemented to provide a custom
// resource owner fire.
type ResourceOwner interface {
	fire.Model

	ResourceOwnerIdentifier() string
	ValidPassword(string) bool
}

// User is the built-in model used to store resource owners.
type User struct {
	fire.Base    `json:"-" bson:",inline" fire:"users"`
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
