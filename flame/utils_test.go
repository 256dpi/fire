package flame

import (
	"net/http"
	"testing"

	"github.com/256dpi/xo"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

var mongoStore = coal.MustConnect("mongodb://0.0.0.0/test-fire-flame", xo.Panic)
var lungoStore = coal.MustOpen(nil, "test-fire-flame", xo.Panic)

var modelList = []coal.Model{&User{}, &Application{}, &Token{}}

var testNotary = heat.NewNotary("test", heat.MustRand(32))

func init() {
	heat.UnsafeFastHash()
}

func withTester(t *testing.T, fn func(*testing.T, *fire.Tester)) {
	t.Run("Mongo", func(t *testing.T) {
		tester := fire.NewTester(mongoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})

	t.Run("Lungo", func(t *testing.T) {
		tester := fire.NewTester(lungoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})
}

func newHandler(auth *Authenticator, force bool) http.Handler {
	router := http.NewServeMux()
	router.Handle("/oauth2/", auth.Endpoint("/oauth2/"))

	authorizer := auth.Authorizer([]string{"foo"}, force, true, true)
	router.Handle("/api/protected", authorizer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})))

	return router
}
