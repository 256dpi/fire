package heat

import (
	"time"

	"github.com/256dpi/xo"
	"github.com/golang-jwt/jwt/v4"

	"github.com/256dpi/fire/stick"
)

const minSecretLen = 16

var jwtSigningMethod = jwt.SigningMethodHS256

var jwtParser = jwt.NewParser(jwt.WithValidMethods([]string{jwtSigningMethod.Name}))

type jwtClaims struct {
	Issuer    string    `json:"iss,omitempty"`
	Audience  string    `json:"aud,omitempty"`
	ID        string    `json:"jti,omitempty"`
	Subject   string    `json:"sub,omitempty"`
	Issued    int64     `json:"iat,omitempty"`
	NotBefore int64     `json:"nbf,omitempty"`
	Expires   int64     `json:"exp,omitempty"`
	Data      stick.Map `json:"dat,omitempty"`
}

func (c jwtClaims) Valid() error {
	// collect errors
	err := &jwt.ValidationError{}

	// check issuer
	if c.Issuer == "" {
		err.Errors |= jwt.ValidationErrorIssuer
		err.Inner = xo.F("missing issuer")
	}

	// check audience
	if c.Audience == "" {
		err.Errors |= jwt.ValidationErrorAudience
		err.Inner = xo.F("missing audience")
	}

	// check id
	if c.ID == "" {
		err.Errors |= jwt.ValidationErrorId
		err.Inner = xo.F("missing id")
	}

	// skip subject

	// get time
	now := time.Now().Unix()

	// check issued
	if c.Issued > now {
		err.Errors |= jwt.ValidationErrorNotValidYet
		err.Inner = xo.F("used before issued")
	}

	// skip not before

	// check expire
	if c.Expires < now {
		err.Errors |= jwt.ValidationErrorExpired
		err.Inner = xo.F("expired")
	}

	// skip data

	// check error
	if err.Errors == 0 {
		return nil
	}

	return err
}

// ErrInvalidToken is returned if a token is invalid.
var ErrInvalidToken = xo.BF("invalid token")

// ErrExpiredToken is returned if a token is expired but otherwise valid.
var ErrExpiredToken = xo.BF("expired token")

// RawKey represents a raw key.
type RawKey struct {
	ID     string
	Expiry time.Time
	Data   stick.Map
}

// Issue will sign a token from the specified raw key.
func Issue(secret []byte, issuer, name string, key RawKey) (string, error) {
	// check secret
	if len(secret) < minSecretLen {
		return "", xo.F("secret too small")
	}

	// check issuer
	if issuer == "" {
		return "", xo.F("missing issuer")
	}

	// check name
	if name == "" {
		return "", xo.F("missing name")
	}

	// check id
	if key.ID == "" {
		return "", xo.F("missing id")
	}

	// check expiry
	if key.Expiry.IsZero() {
		return "", xo.F("missing expiry")
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
		return "", xo.W(err)
	}

	return sig, nil
}

// Verify will verify the specified token and return the decoded raw key.
func Verify(secret []byte, issuer, name, token string) (*RawKey, error) {
	// check secret
	if len(secret) < minSecretLen {
		return nil, xo.F("secret too small")
	}

	// check issuer
	if issuer == "" {
		return nil, xo.F("missing issuer")
	}

	// check name
	if name == "" {
		return nil, xo.F("missing name")
	}

	// parse token
	var claims jwtClaims
	tkn, err := jwtParser.ParseWithClaims(token, &claims, func(_ *jwt.Token) (interface{}, error) {
		return secret, nil
	})
	if valErr, ok := err.(*jwt.ValidationError); ok && valErr != nil {
		if valErr.Errors == jwt.ValidationErrorExpired {
			return nil, ErrExpiredToken.Wrap()
		}
		return nil, ErrInvalidToken.Wrap()
	} else if err != nil {
		return nil, xo.W(err)
	} else if !tkn.Valid {
		return nil, ErrInvalidToken.Wrap()
	}

	// check issuer
	if claims.Issuer != issuer {
		return nil, ErrInvalidToken.Wrap()
	}

	// check name
	if claims.Audience != name {
		return nil, ErrInvalidToken.Wrap()
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
