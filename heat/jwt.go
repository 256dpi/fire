package heat

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

var jwtSigningMethod = jwt.SigningMethodHS256

var jwtParser = &jwt.Parser{
	ValidMethods: []string{jwtSigningMethod.Name},
}

type jwtClaims struct {
	jwt.StandardClaims
	Data Data `json:"dat,omitempty"`
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

// Verify will verify the specified token and return the decoded raw key.
func Verify(secret []byte, issuer, name, token string) (*RawKey, error) {
	// check name
	if name == "" {
		panic("heat: missing name")
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

	// check id
	if claims.Id == "" {
		return nil, ErrInvalidToken
	}

	// get expiry
	expiry := time.Unix(claims.ExpiresAt, 0)

	// prepare key
	key := &RawKey{
		ID:     claims.Id,
		Expiry: expiry,
		Data:   claims.Data,
	}

	return key, nil
}

// Issue will sign a token from the specified raw key.
func Issue(secret []byte, issuer, name string, key RawKey) (string, error) {
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
		StandardClaims: jwt.StandardClaims{
			Issuer:   issuer,
			Audience: name,
			Id:       key.ID,
			// Subject:   "",
			IssuedAt: now.Unix(),
			// NotBefore: 0,
			ExpiresAt: key.Expiry.Unix(),
		},
		Data: key.Data,
	})

	// compute signature
	sig, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}

	return sig, nil
}
