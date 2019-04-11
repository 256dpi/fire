package flame

import (
	"time"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/asaskevich/govalidator"
	"github.com/globalsign/mgo/bson"
	"golang.org/x/crypto/bcrypt"
)

// TokenType defines the type of a token.
type TokenType string

const (
	// AccessToken defines an access token.
	AccessToken TokenType = "access"

	// RefreshToken defines a refresh token.
	RefreshToken TokenType = "refresh"
)

// GenericToken is the interface that must be implemented by the tokens.
type GenericToken interface {
	coal.Model

	// GetTokenData should collect and return the tokens data.
	GetTokenData() (typ TokenType, scope []string, expiresAt time.Time, client bson.ObjectId, resourceOwner *bson.ObjectId)

	// SetTokenData should set the specified token data.
	SetTokenData(typ TokenType, scope []string, expiresAt time.Time, client Client, resourceOwner ResourceOwner)
}

// Token is the built-in model used to store access and refresh tokens.
type Token struct {
	coal.Base     `json:"-" bson:",inline" coal:"tokens:tokens"`
	Type          TokenType      `json:"type"`
	ExpiresAt     time.Time      `json:"expires-at" bson:"expires_at"`
	Scope         []string       `json:"scope" bson:"scope"`
	Client        bson.ObjectId  `json:"client-id" bson:"client_id"`
	ResourceOwner *bson.ObjectId `json:"resource-owner-id" bson:"resource_owner_id"`
}

// AddTokenIndexes will add access token indexes to the specified indexer.
func AddTokenIndexes(i *coal.Indexer, autoExpire bool) {
	i.Add(&Token{}, false, 0, "Type")
	i.Add(&Token{}, false, 0, "Client")
	i.Add(&Token{}, false, 0, "ResourceOwner")

	if autoExpire {
		i.Add(&Token{}, false, time.Minute, "ExpiresAt")
	}
}

// GetTokenData implements the flame.GenericToken interface.
func (t *Token) GetTokenData() (TokenType, []string, time.Time, bson.ObjectId, *bson.ObjectId) {
	return t.Type, t.Scope, t.ExpiresAt, t.Client, t.ResourceOwner
}

// SetTokenData implements the flame.GenericToken interface.
func (t *Token) SetTokenData(typ TokenType, scope []string, expiresAt time.Time, client Client, resourceOwner ResourceOwner) {
	t.Type = typ
	t.Scope = scope
	t.ExpiresAt = expiresAt
	t.Client = client.ID()
	if resourceOwner != nil {
		t.ResourceOwner = coal.P(resourceOwner.ID())
	}
}

// Validate implements the fire.ValidatableModel interface.
func (t *Token) Validate() error {
	// check id
	if !t.ID().Valid() {
		return fire.E("invalid id")
	}

	// check expires at
	if t.ExpiresAt.IsZero() {
		return fire.E("expires at not set")
	}

	return nil
}

// Client is the interface that must be implemented by clients.
type Client interface {
	coal.Model

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
	coal.Base   `json:"-" bson:",inline" coal:"applications"`
	Name        string `json:"name" bson:"name"`
	Key         string `json:"key" bson:"key" coal:"flame-client-id"`
	Secret      string `json:"secret,omitempty" bson:"-"`
	SecretHash  []byte `json:"-" bson:"secret"`
	RedirectURL string `json:"redirect-url" bson:"redirect_url"`
}

// AddApplicationIndexes will add application indexes to the specified indexer.
func AddApplicationIndexes(i *coal.Indexer) {
	i.Add(&Application{}, true, 0, "Key")
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

	// check id
	if !a.ID().Valid() {
		return fire.E("invalid id")
	}

	// check name
	if a.Name == "" {
		return fire.E("name not set")
	}

	// check key
	if a.Key == "" {
		return fire.E("key not set")
	}

	// check secret hash
	if len(a.SecretHash) == 0 {
		return fire.E("secret hash not set")
	}

	// check redirect uri
	if a.RedirectURL != "" && !govalidator.IsURL(a.RedirectURL) {
		return fire.E("invalid redirect url")
	}

	return nil
}

// HashSecret will hash Secret and set SecretHash.
func (a *Application) HashSecret() error {
	// check length
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

// ResourceOwner is the interface that must be implemented resource owners.
type ResourceOwner interface {
	coal.Model

	// ValidSecret should determine whether the specified plain text password
	// matches the stored hashed password.
	ValidPassword(string) bool
}

// User is the built-in model used to store resource owners.
type User struct {
	coal.Base    `json:"-" bson:",inline" coal:"users"`
	Name         string `json:"name" bson:"name"`
	Email        string `json:"email" bson:"email" coal:"flame-resource-owner-id"`
	Password     string `json:"password,omitempty" bson:"-"`
	PasswordHash []byte `json:"-" bson:"password"`
}

// AddUserIndexes will add user indexes to the specified indexer.
func AddUserIndexes(i *coal.Indexer) {
	i.Add(&User{}, true, 0, "Email")
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

	// check id
	if !u.ID().Valid() {
		return fire.E("invalid id")
	}

	// check name
	if u.Name == "" {
		return fire.E("name not set")
	}

	// check email
	if u.Email == "" || !govalidator.IsEmail(u.Email) {
		return fire.E("invalid email")
	}

	// check password hash
	if len(u.PasswordHash) == 0 {
		return fire.E("password hash not set")
	}

	return nil
}

// HashPassword will hash Password and set PasswordHash.
func (u *User) HashPassword() error {
	// check length
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
