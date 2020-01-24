package heat

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const minSecretLen = 16

var jwtSigningMethod = jwt.SigningMethodHS256

var jwtParser = &jwt.Parser{
	ValidMethods: []string{jwtSigningMethod.Name},
}

type jwtClaims struct {
	Issuer    string `json:"iss,omitempty"`
	Audience  string `json:"aud,omitempty"`
	ID        string `json:"jti,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Issued    int64  `json:"iat,omitempty"`
	NotBefore int64  `json:"nbf,omitempty"`
	Expires   int64  `json:"exp,omitempty"`
	Data      Data   `json:"dat,omitempty"`
}

func (c jwtClaims) Valid() error {
	// collect errors
	err := &jwt.ValidationError{}

	// check issuer
	if c.Issuer == "" {
		err.Errors |= jwt.ValidationErrorIssuer
		err.Inner = fmt.Errorf("missing issuer")
	}

	// check audience
	if c.Audience == "" {
		err.Errors |= jwt.ValidationErrorAudience
		err.Inner = fmt.Errorf("missing audience")
	}

	// check id
	if c.ID == "" {
		err.Errors |= jwt.ValidationErrorId
		err.Inner = fmt.Errorf("missing id")
	}

	// skip subject

	// get time
	now := time.Now().Unix()

	// check issued
	if c.Issued > now {
		err.Errors |= jwt.ValidationErrorNotValidYet
		err.Inner = fmt.Errorf("used before issued")
	}

	// skip not before

	// check expire
	if c.Expires < now {
		err.Errors |= jwt.ValidationErrorExpired
		err.Inner = fmt.Errorf("expired")
	}

	// skip data

	// check error
	if err.Errors == 0 {
		return nil
	}

	return err
}

// ErrInvalidToken is returned if a token is in some way invalid.
var ErrInvalidToken = errors.New("invalid token")

// ErrExpiredToken is returned if a token is expired but otherwise valid.
var ErrExpiredToken = errors.New("expired token")

// Data is generic JSON object.
type Data map[string]interface{}

// RawKey represents a raw key.
type RawKey struct {
	ID     string
	Expiry time.Time
	Data   Data
}

// Issue will sign a token from the specified raw key.
func Issue(secret []byte, issuer, name string, key RawKey) (string, error) {
	// check secret
	if len(secret) < minSecretLen {
		return "", fmt.Errorf("secret too small")
	}

	// check issuer
	if issuer == "" {
		return "", fmt.Errorf("missing issuer")
	}

	// check name
	if name == "" {
		return "", fmt.Errorf("missing name")
	}

	// check id
	if key.ID == "" {
		return "", fmt.Errorf("missing id")
	}

	// check expiry
	if key.Expiry.IsZero() {
		return "", fmt.Errorf("missing expiry")
	}

	// get time
	now := time.Now()

	// create token
	token := jwt.NewWithClaims(jwtSigningMethod, jwtClaims{
		Issuer:   issuer,
		Audience: name,
		ID:       key.ID,
		// Subject:   "",
		Issued: now.Unix(),
		// NotBefore: 0,
		Expires: key.Expiry.Unix(),
		Data:    key.Data,
	})

	// compute signature
	sig, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return sig, nil
}

// Verify will verify the specified token and return the decoded raw key.
func Verify(secret []byte, issuer, name, token string) (*RawKey, error) {
	// check secret
	if len(secret) < minSecretLen {
		return nil, fmt.Errorf("secret too small")
	}

	// check issuer
	if issuer == "" {
		return nil, fmt.Errorf("missing issuer")
	}

	// check name
	if name == "" {
		return nil, fmt.Errorf("missing name")
	}

	// parse token
	var claims jwtClaims
	tkn, err := jwtParser.ParseWithClaims(token, &claims, func(_ *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if valErr, ok := err.(*jwt.ValidationError); ok && valErr != nil {
		if valErr.Errors == jwt.ValidationErrorExpired {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	} else if err != nil {
		return nil, err
	} else if !tkn.Valid {
		return nil, ErrInvalidToken
	}

	// check issuer
	if claims.Issuer != issuer {
		return nil, ErrInvalidToken
	}

	// check name
	if claims.Audience != name {
		return nil, ErrInvalidToken
	}

	// get expiry
	expiry := time.Unix(claims.Expires, 0)

	// prepare key
	key := &RawKey{
		ID:     claims.ID,
		Expiry: expiry,
		Data:   claims.Data,
	}

	return key, nil
}
