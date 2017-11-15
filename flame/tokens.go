package flame

import (
	"time"

	"github.com/dgrijalva/jwt-go"
	"gopkg.in/mgo.v2/bson"
)

type accessTokenClaims struct {
	jwt.StandardClaims
	Data map[string]interface{} `json:"dat"`
}

func generateAccessToken(id bson.ObjectId, secret []byte, issuedAt, expiresAt time.Time, ro ResourceOwner) (string, error) {
	// prepare claims
	claims := &accessTokenClaims{}
	claims.Id = id.Hex()
	claims.IssuedAt = issuedAt.Unix()
	claims.ExpiresAt = expiresAt.Unix()

	// set user data
	if ro != nil {
		claims.Data = ro.DataForAccessToken()
	}

	// create token
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// sign token
	str, err := tkn.SignedString(secret)
	if err != nil {
		return "", nil
	}

	return str, nil
}

type refreshTokenClaims struct {
	jwt.StandardClaims
}

func generateRefreshToken(id bson.ObjectId, secret []byte, issuedAt, expiresAt time.Time) (string, error) {
	// prepare claims
	claims := &refreshTokenClaims{}
	claims.Id = id.Hex()
	claims.IssuedAt = issuedAt.Unix()
	claims.ExpiresAt = expiresAt.Unix()

	// create token
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// sign token
	str, err := tkn.SignedString(secret)
	if err != nil {
		return "", nil
	}

	return str, nil
}
