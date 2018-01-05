package flame

import (
	"time"

	"github.com/256dpi/fire/coal"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// TokenData is used to carry token related information.
type TokenData struct {
	Scope           []string
	ExpiresAt       time.Time
	ClientID        bson.ObjectId
	ResourceOwnerID *bson.ObjectId
}

// A TokenDescription is returned by a Token model to specify details about
// its implementation.
type TokenDescription struct {
	ClientIDField  string
	ExpiresAtField string
}

// Token is the interface that must be implemented by the tokens.
type Token interface {
	coal.Model

	// DescribeToken should return a TokenDescription.
	DescribeToken() TokenDescription

	// GetTokenData should collect and return the tokens data.
	GetTokenData() *TokenData

	// SetTokenData should set the specified token data.
	SetTokenData(*TokenData)
}

// AccessToken is the built-in model used to store access tokens.
type AccessToken struct {
	coal.Base     `json:"-" bson:",inline" valid:"required" coal:"access-tokens:access_tokens"`
	ExpiresAt     time.Time      `json:"expires-at" valid:"required" bson:"expires_at"`
	Scope         []string       `json:"scope" valid:"required" bson:"scope"`
	Client        bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id"`
	ResourceOwner *bson.ObjectId `json:"resource-owner-id" valid:"-" bson:"resource_owner_id"`
}

// AddAccessTokenIndexes will add access token indexes to the specified indexer.
func AddAccessTokenIndexes(i *coal.Indexer, autoExpire bool) {
	i.Add(&AccessToken{}, false, false, "Client")
	i.Add(&AccessToken{}, false, false, "ResourceOwner")

	if autoExpire {
		i.AddRaw(coal.C(&AccessToken{}), mgo.Index{
			Key:         []string{coal.F(&AccessToken{}, "ExpiresAt")},
			ExpireAfter: time.Minute,
			Background:  true,
		})
	}
}

// DescribeToken implements the flame.Token interface.
func (t *AccessToken) DescribeToken() TokenDescription {
	return TokenDescription{
		ClientIDField:  "Client",
		ExpiresAtField: "ExpiresAt",
	}
}

// GetTokenData implements the flame.Token interface.
func (t *AccessToken) GetTokenData() *TokenData {
	return &TokenData{
		Scope:           t.Scope,
		ExpiresAt:       t.ExpiresAt,
		ClientID:        t.Client,
		ResourceOwnerID: t.ResourceOwner,
	}
}

// SetTokenData implements the flame.Token interface.
func (t *AccessToken) SetTokenData(data *TokenData) {
	t.Scope = data.Scope
	t.ExpiresAt = data.ExpiresAt
	t.Client = data.ClientID
	t.ResourceOwner = data.ResourceOwnerID
}

// RefreshToken is the built-in model used to store refresh tokens.
type RefreshToken struct {
	coal.Base     `json:"-" bson:",inline" valid:"required" coal:"refresh-tokens:refresh_tokens"`
	ExpiresAt     time.Time      `json:"expires-at" valid:"required" bson:"expires_at"`
	Scope         []string       `json:"scope" valid:"required" bson:"scope"`
	Client        bson.ObjectId  `json:"client-id" valid:"-" bson:"client_id"`
	ResourceOwner *bson.ObjectId `json:"resource-owner-id" valid:"-" bson:"resource_owner_id"`
}

// AddRefreshTokenIndexes will add refresh token indexes to the specified indexer.
func AddRefreshTokenIndexes(i *coal.Indexer, autoExpire bool) {
	i.Add(&RefreshToken{}, false, false, "Client")
	i.Add(&RefreshToken{}, false, false, "ResourceOwner")

	if autoExpire {
		i.AddRaw(coal.C(&RefreshToken{}), mgo.Index{
			Key:         []string{coal.F(&RefreshToken{}, "ExpiresAt")},
			ExpireAfter: time.Minute,
			Background:  true,
		})
	}
}

// DescribeToken implements the flame.Token interface.
func (t *RefreshToken) DescribeToken() TokenDescription {
	return TokenDescription{
		ClientIDField:  "Client",
		ExpiresAtField: "ExpiresAt",
	}
}

// GetTokenData implements the flame.Token interface.
func (t *RefreshToken) GetTokenData() *TokenData {
	return &TokenData{
		Scope:           t.Scope,
		ExpiresAt:       t.ExpiresAt,
		ClientID:        t.Client,
		ResourceOwnerID: t.ResourceOwner,
	}
}

// SetTokenData implements the flame.Token interface.
func (t *RefreshToken) SetTokenData(data *TokenData) {
	t.Scope = data.Scope
	t.ExpiresAt = data.ExpiresAt
	t.Client = data.ClientID
	t.ResourceOwner = data.ResourceOwnerID
}

// A ClientDescription is returned by a Client model to specify details about
// its implementation.
type ClientDescription struct {
	IdentifierField string
}

// Client is the interface that must be implemented by clients.
type Client interface {
	coal.Model

	// DescribeClient should return a ClientDescription.
	DescribeClient() ClientDescription

	// ValidRedirectURL should return whether the specified redirect url can be
	// used by this client.
	//
	// Note: In order to increases security the callback should only allow
	// pre-registered redirect urls.
	ValidRedirectURL(string) bool

	// ValidSecret should determine whether the specified plain text secret
	// matches the stored hashed secret.
	ValidSecret(string) bool
}

// Application is the built-in model used to store clients.
type Application struct {
	coal.Base   `json:"-" bson:",inline" valid:"required" coal:"applications"`
	Name        string `json:"name" valid:"required"`
	Key         string `json:"key" valid:"required"`
	Secret      string `json:"secret,omitempty" bson:"-"`
	SecretHash  []byte `json:"-" valid:"required"`
	RedirectURL string `json:"redirect_url" valid:"required,url"`
}

// AddApplicationIndexes will add application indexes to the specified indexer.
func AddApplicationIndexes(i *coal.Indexer) {
	i.Add(&Application{}, true, false, "Key")
}

// DescribeClient implements the flame.Client interface.
func (a *Application) DescribeClient() ClientDescription {
	return ClientDescription{
		IdentifierField: "Key",
	}
}

// ValidRedirectURL implements the flame.Client interface.
func (a *Application) ValidRedirectURL(url string) bool {
	return url == a.RedirectURL
}

// ValidSecret implements the flame.Client interface.
func (a *Application) ValidSecret(secret string) bool {
	return bcrypt.CompareHashAndPassword(a.SecretHash, []byte(secret)) == nil
}

// Validate implements the coal.ValidatableModel interface.
func (a *Application) Validate() error {
	// hash password if available
	err := a.HashSecret()
	if err != nil {
		return err
	}

	return nil
}

// HashSecret will hash Secret and set SecretHash.
func (a *Application) HashSecret() error {
	if len(a.Secret) == 0 {
		return nil
	}

	// generate hash from password
	hash, err := bcrypt.GenerateFromPassword([]byte(a.Secret), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// save hash
	a.SecretHash = hash

	// clear password
	a.Secret = ""

	return nil
}

// A ResourceOwnerDescription is returned by a ResourceOwner model to specify
// details about its implementation.
type ResourceOwnerDescription struct {
	IdentifierField string
}

// ResourceOwner is the interface that must be implemented resource owners.
type ResourceOwner interface {
	coal.Model

	// DescribeResourceOwner should return a ResourceOwnerDescription.
	DescribeResourceOwner() ResourceOwnerDescription

	// ValidSecret should determine whether the specified plain text password
	// matches the stored hashed password.
	ValidPassword(string) bool
}

// User is the built-in model used to store resource owners.
type User struct {
	coal.Base    `json:"-" bson:",inline" valid:"required" coal:"users"`
	Name         string `json:"name" valid:"required"`
	Email        string `json:"email" valid:"required,email"`
	Password     string `json:"password,omitempty" bson:"-"`
	PasswordHash []byte `json:"-" valid:"required"`
}

// AddUserIndexes will add user indexes to the specified indexer.
func AddUserIndexes(i *coal.Indexer) {
	i.Add(&User{}, true, false, "Email")
}

// DescribeResourceOwner implements the flame.ResourceOwner interface.
func (u *User) DescribeResourceOwner() ResourceOwnerDescription {
	return ResourceOwnerDescription{
		IdentifierField: "Email",
	}
}

// ValidPassword implements the flame.ResourceOwner interface.
func (u *User) ValidPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)) == nil
}

// Validate implements the coal.ValidatableModel interface.
func (u *User) Validate() error {
	// hash password if available
	err := u.HashPassword()
	if err != nil {
		return err
	}

	return nil
}

// HashPassword will hash Password and set PasswordHash.
func (u *User) HashPassword() error {
	if len(u.Password) == 0 {
		return nil
	}

	// generate hash from password
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// save hash
	u.PasswordHash = hash

	// clear password
	u.Password = ""

	return nil
}
