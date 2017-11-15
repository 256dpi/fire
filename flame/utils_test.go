package flame

import (
	"net/http"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"golang.org/x/crypto/bcrypt"
)

var tester = fire.NewTester(coal.MustCreateStore("mongodb://0.0.0.0:27017/test-flame"), &User{}, &Application{}, &AccessToken{}, &RefreshToken{})

func newHandler(auth *Authenticator, force bool) http.Handler {
	router := http.NewServeMux()
	router.Handle("/oauth2/", auth.Endpoint("/oauth2/"))

	authorizer := auth.Authorizer("foo", force)
	router.Handle("/api/protected", authorizer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})))

	return router
}

func mustHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	return hash
}
