package flame

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestGenerateJWTAndParseJWT(t *testing.T) {
	id := coal.New().Hex()

	str, err := GenerateJWT("foo", Claims{
		StandardClaims: jwt.StandardClaims{Id: id},
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, str)

	token, claims, err := ParseJWT("foo", str)
	assert.NoError(t, err)
	assert.Equal(t, &jwt.Token{
		Raw:    str,
		Method: jwt.SigningMethodHS256,
		Header: map[string]interface{}{
			"alg": "HS256",
			"typ": "JWT",
		},
		Claims: &Claims{
			StandardClaims: jwt.StandardClaims{Id: id},
		},
		Signature: strings.Split(str, ".")[2],
		Valid:     true,
	}, token)
	assert.Equal(t, &Claims{
		StandardClaims: jwt.StandardClaims{Id: id},
	}, claims)
}

func TestParseJWTInvalidSigningMethod(t *testing.T) {
	str, err := jwt.New(jwt.SigningMethodHS384).SignedString([]byte("foo"))
	assert.NoError(t, err)
	assert.NotEmpty(t, str)

	token, claims, err := ParseJWT("foo", str)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Nil(t, token)
}

func TestTokenMigrator(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		migrator := TokenMigrator(true)

		tester.Handler = migrator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer foo", r.Header.Get("Authorization"))
			assert.Equal(t, "", r.URL.Query().Get("access_token"))

			_, _ = w.Write([]byte("OK"))
		}))

		tester.Request("GET", "foo?access_token=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, "OK", r.Body.String())
		})
	})
}

func TestEnsureApplicationAndGetApplicationKey(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		key, err := EnsureApplication(tester.Store, "Foo", "bar", "baz")
		assert.NoError(t, err)
		assert.Equal(t, "bar", key)

		app := tester.FindLast(&Application{}).(*Application)
		assert.Equal(t, "Foo", app.Name)
		assert.Equal(t, "bar", app.Key)
		assert.Empty(t, app.Secret)
		assert.NotEmpty(t, app.SecretHash)
	})
}

func TestEnsureFirstUser(t *testing.T) {
	withTester(t, func(t *testing.T, tester *fire.Tester) {
		err := EnsureFirstUser(tester.Store, "Foo", "foo@bar.com", "bar")
		assert.NoError(t, err)

		user := tester.FindLast(&User{}).(*User)
		assert.Equal(t, "Foo", user.Name)
		assert.Equal(t, "foo@bar.com", user.Email)
		assert.Empty(t, user.Password)
		assert.NotEmpty(t, user.PasswordHash)
	})
}
