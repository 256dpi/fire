package flame

import (
	"time"

	"github.com/256dpi/oauth2/v2"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
)

// TokenType defines the token type.
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
	Scope oauth2.Scope

	// The token expiry.
	ExpiresAt time.Time

	// The stored redirect URI.
	RedirectURI string

	// The client and resource owner models.
	//
	// Mandatory for `SetTokenData` optional for `GetTokenData`.
	Client        Client
	ResourceOwner ResourceOwner

	// The client and resource owner IDs.
	ClientID        coal.ID
	ResourceOwnerID *coal.ID
}

// GenericToken is the interface that must be implemented by tokens.
type GenericToken interface {
	coal.Model

	// GetTokenData should collect and return the token data.
	GetTokenData() TokenData

	// SetTokenData should apply the specified token data.
	SetTokenData(TokenData)
}

func init() {
	// add indexes
	coal.AddIndex(&Token{}, false, 0, "Type")
	coal.AddIndex(&Token{}, false, 0, "Application")
	coal.AddIndex(&Token{}, false, 0, "User")
	coal.AddIndex(&Token{}, false, time.Minute, "ExpiresAt")
}

// Token is the built-in model used to store access, refresh tokens and
// authorization codes.
type Token struct {
	coal.Base   `json:"-" bson:",inline" coal:"tokens:tokens"`
	Type        TokenType `json:"type"`
	Scope       []string  `json:"scope"`
	ExpiresAt   time.Time `json:"expires-at" bson:"expires_at"`
	RedirectURI string    `json:"redirect-uri" bson:"redirect_uri"`
	Application coal.ID   `json:"-" bson:"application_id" coal:"application:applications"`
	User        *coal.ID  `json:"-" bson:"user_id" coal:"user:users"`
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
		t.User = stick.P(data.ResourceOwner.ID())
	}
}

// Validate implements the fire.ValidatableModel interface.
func (t *Token) Validate() error {
	return stick.Validate(t, func(v *stick.Validator) {
		v.Value("Type", false, stick.IsNotZero, stick.IsValidUTF8)
		v.Items("Scope", stick.IsNotZero, stick.IsValidUTF8)
		v.Value("ExpiresAt", false, stick.IsNotZero)
		v.Value("RedirectURI", false, stick.IsValidUTF8)
		v.Value("Application", false, stick.IsNotZero)
		v.Value("User", true, stick.IsNotZero)
	})
}

// Client is the interface that must be implemented by clients. The field used
// to uniquely identify the client may be flagged with "flame-client-id". If
// missing the model ID is used instead.
type Client interface {
	coal.Model

	// IsConfidential returns whether the client should be treated as a
	// confidential client that has been issue client credentials for
	// authenticating itself.
	IsConfidential() bool

	// ValidRedirectURI should return whether the specified redirect URI can be
	// used by this client.
	//
	// Note: In order to increase security the callback should only allow
	// pre-registered redirect URIs.
	ValidRedirectURI(string) bool

	// ValidSecret should determine whether the specified plain text secret
	// matches the stored hashed secret.
	ValidSecret(string) bool
}

func init() {
	// add index
	coal.AddIndex(&Application{}, true, 0, "Key")
}

// Application is the built-in model used to store clients.
type Application struct {
	coal.Base    `json:"-" bson:",inline" coal:"applications"`
	Name         string   `json:"name"`
	Key          string   `json:"key" coal:"flame-client-id"`
	Secret       string   `json:"secret,omitempty" bson:"-"`
	SecretHash   []byte   `json:"-" bson:"secret"`
	RedirectURIs []string `json:"redirect-uris" bson:"redirect_uris"`
}

// IsConfidential implements the flame.Client interface.
func (a *Application) IsConfidential() bool {
	return len(a.SecretHash) > 0
}

// ValidRedirectURI implements the flame.Client interface.
func (a *Application) ValidRedirectURI(uri string) bool {
	return stick.Contains(a.RedirectURIs, uri)
}

// ValidSecret implements the flame.Client interface.
func (a *Application) ValidSecret(secret string) bool {
	return heat.Compare(a.SecretHash, secret) == nil
}

// Validate implements the fire.ValidatableModel interface.
func (a *Application) Validate() error {
	// hash password if available
	err := a.HashSecret()
	if err != nil {
		return err
	}

	return stick.Validate(a, func(v *stick.Validator) {
		v.Value("Name", false, stick.IsNotZero, stick.IsValidUTF8)
		v.Value("Key", false, stick.IsNotZero, stick.IsValidUTF8)
		v.Items("RedirectURIs", stick.IsNotZero, stick.IsValidUTF8)
	})
}

// HashSecret will hash Secret and set SecretHash.
func (a *Application) HashSecret() error {
	// check length
	if len(a.Secret) == 0 {
		return nil
	}

	// generate hash from password
	hash, err := heat.Hash(a.Secret)
	if err != nil {
		return err
	}

	// save hash
	a.SecretHash = hash

	// clear password
	a.Secret = ""

	return nil
}

// ResourceOwner is the interface that must be implemented by resource owners.
// The field used to uniquely identify the resource owner may be flagged with
// "flame-resource-owner-id". If missing the model ID is used instead.
type ResourceOwner interface {
	coal.Model

	// ValidPassword should determine whether the specified plain text password
	// matches the stored hashed password.
	ValidPassword(string) bool
}

func init() {
	// add index
	coal.AddIndex(&User{}, true, 0, "Email")
}

// User is the built-in model used to store resource owners.
type User struct {
	coal.Base    `json:"-" bson:",inline" coal:"users"`
	Name         string `json:"name"`
	Email        string `json:"email" coal:"flame-resource-owner-id"`
	Password     string `json:"password,omitempty" bson:"-"`
	PasswordHash []byte `json:"-" bson:"password"`
}

// ValidPassword implements the flame.ResourceOwner interface.
func (u *User) ValidPassword(password string) bool {
	return heat.Compare(u.PasswordHash, password) == nil
}

// Validate implements the fire.ValidatableModel interface.
func (u *User) Validate() error {
	// hash password if available
	err := u.HashPassword()
	if err != nil {
		return err
	}

	return stick.Validate(u, func(v *stick.Validator) {
		v.Value("Name", false, stick.IsNotZero, stick.IsValidUTF8)
		v.Value("Email", false, stick.IsNotZero, stick.IsEmail)
		v.Value("PasswordHash", false, stick.IsNotEmpty)
	})
}

// HashPassword will hash Password and set PasswordHash.
func (u *User) HashPassword() error {
	// check length
	if len(u.Password) == 0 {
		return nil
	}

	// generate hash from password
	hash, err := heat.Hash(u.Password)
	if err != nil {
		return err
	}

	// save hash
	u.PasswordHash = hash

	// clear password
	u.Password = ""

	return nil
}
