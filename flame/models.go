package flame

import (
	"time"

	"github.com/asaskevich/govalidator"
	"golang.org/x/crypto/bcrypt"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

// TokenType defines the type of a token.
type TokenType string

const (
	// AccessToken defines an access token.
	AccessToken TokenType = "access"

	// RefreshToken defines a refresh token.
	RefreshToken TokenType = "refresh"

	// AuthorizationCode defines an authorization code.
	AuthorizationCode TokenType = "code"
)

// TokenData describes attributes of a token.
type TokenData struct {
	// The token type.
	Type TokenType

	// The token scope.
	Scope []string

	// The token expiry.
	ExpiresAt time.Time

	// The stored redirect URI.
	RedirectURI string

	// The client and resource owner model.
	//
	// Mandatory for `SetTokenData` optional for `GetTokenData`.
	Client        Client
	ResourceOwner ResourceOwner

	// The client and resource owner id.
	ClientID        coal.ID
	ResourceOwnerID *coal.ID
}

// GenericToken is the interface that must be implemented by the tokens.
type GenericToken interface {
	coal.Model

	// GetTokenData should collect and return the tokens data.
	GetTokenData() TokenData

	// SetTokenData should set the specified token data.
	SetTokenData(TokenData)
}

// Token is the built-in model used to store access and refresh tokens.
type Token struct {
	coal.Base   `json:"-" bson:",inline" coal:"tokens:tokens"`
	Type        TokenType `json:"type"`
	Scope       []string  `json:"scope" bson:"scope"`
	ExpiresAt   time.Time `json:"expires-at" bson:"expires_at"`
	RedirectURI string    `json:"redirect-uri" bson:"redirect_uri"`
	Application coal.ID   `json:"-" bson:"application_id" coal:"application:applications"`
	User        *coal.ID  `json:"-" bson:"user_id" coal:"user:users"`
}

// AddTokenIndexes will add access token indexes to the specified indexer.
func AddTokenIndexes(i *coal.Indexer, autoExpire bool) {
	i.Add(&Token{}, false, 0, "Type")
	i.Add(&Token{}, false, 0, "Application")
	i.Add(&Token{}, false, 0, "User")

	if autoExpire {
		i.Add(&Token{}, false, time.Minute, "ExpiresAt")
	}
}

// GetTokenData implements the flame.GenericToken interface.
func (t *Token) GetTokenData() TokenData {
	return TokenData{
		Type:            t.Type,
		Scope:           t.Scope,
		ExpiresAt:       t.ExpiresAt,
		RedirectURI:     t.RedirectURI,
		ClientID:        t.Application,
		ResourceOwnerID: t.User,
	}
}

// SetTokenData implements the flame.GenericToken interface.
func (t *Token) SetTokenData(data TokenData) {
	t.Type = data.Type
	t.Scope = data.Scope
	t.ExpiresAt = data.ExpiresAt
	t.RedirectURI = data.RedirectURI
	t.Application = data.Client.ID()
	if data.ResourceOwner != nil {
		t.User = coal.P(data.ResourceOwner.ID())
	}
}

// Validate implements the fire.ValidatableModel interface.
func (t *Token) Validate() error {
	// check id
	if t.ID().IsZero() {
		return fire.E("invalid id")
	}

	// check expires at
	if t.ExpiresAt.IsZero() {
		return fire.E("expires at not set")
	}

	return nil
}

// Client is the interface that must be implemented by clients. The field used
// to uniquely identify the client must be flagged with "flame-client-id".
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
	coal.Base    `json:"-" bson:",inline" coal:"applications"`
	Name         string   `json:"name" bson:"name"`
	Key          string   `json:"key" bson:"key" coal:"flame-client-id"`
	Secret       string   `json:"secret,omitempty" bson:"-"`
	SecretHash   []byte   `json:"-" bson:"secret"`
	RedirectURLs []string `json:"redirect-urls" bson:"redirect_urls"`
}

// AddApplicationIndexes will add application indexes to the specified indexer.
func AddApplicationIndexes(i *coal.Indexer) {
	i.Add(&Application{}, true, 0, "Key")
}

// ValidRedirectURL implements the flame.Client interface.
func (a *Application) ValidRedirectURL(url string) bool {
	return fire.Contains(a.RedirectURLs, url)
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
	if a.ID().IsZero() {
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
	for _, redirectURL := range a.RedirectURLs {
		if redirectURL != "" && !govalidator.IsURL(redirectURL) {
			return fire.E("invalid redirect url")
		}
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

// ResourceOwner is the interface that must be implemented resource owners. The
// field used to uniquely identify the resource owner must be flagged with
// "flame-resource-owner-id".
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
	if u.ID().IsZero() {
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
