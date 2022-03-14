package heat

import "github.com/golang-jwt/jwt/v4"

var testSecret = MustRand(32)

func makeToken(claims jwtClaims) string {
	token, err := jwt.NewWithClaims(jwtSigningMethod, claims).SignedString(testSecret)
	if err != nil {
		panic(err)
	}

	return token
}
