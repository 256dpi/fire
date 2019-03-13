package spark

import (
	"time"

	"github.com/dgrijalva/jwt-go"
)

// A Policy configures the provided authentication schemes.
type Policy struct {
	// The secret should be at least 16 characters long.
	Secret []byte

	// The token lifespan.
	TokenLifespan time.Duration
}

// TokenClaims represents the data included in a watch token.
type TokenClaims struct {
	jwt.StandardClaims

	// Data contains user defined key value pairs.
	Data map[string]interface{} `json:"dat"`
}

// DefaultPolicy returns a policy.
//
// Note: The secret should be at least 16 characters long.
func DefaultPolicy(secret string) *Policy {
	return &Policy{
		Secret:        []byte(secret),
		TokenLifespan: time.Hour,
	}
}

// GenerateToken returns a new token for the provided information.
func (p *Policy) GenerateToken(sub, id string, issuedAt, expiresAt time.Time, filters map[string]interface{}) (string, error) {
	// prepare claims
	claims := &TokenClaims{}
	claims.Subject = sub
	claims.Id = id
	claims.IssuedAt = issuedAt.Unix()
	claims.ExpiresAt = expiresAt.Unix()

	// set user data
	claims.Data = filters

	// create token
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// sign token
	str, err := tkn.SignedString(p.Secret)
	if err != nil {
		return "", nil
	}

	return str, nil
}

// ParseToken will parse the presented token and return its claims, if it is
// expired and eventual errors.
func (p *Policy) ParseToken(str string) (*TokenClaims, bool, error) {
	// parse token and check id
	var claims TokenClaims
	_, err := jwt.ParseWithClaims(str, &claims, func(_ *jwt.Token) (interface{}, error) {
		return p.Secret, nil
	})
	if valErr, ok := err.(*jwt.ValidationError); ok && valErr.Errors == jwt.ValidationErrorExpired {
		return nil, true, err
	} else if err != nil {
		return nil, false, err
	}

	return &claims, false, nil
}
