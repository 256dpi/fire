package flame

import (
	"net/http"
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
	"golang.org/x/crypto/bcrypt"
)

var tester = fire.NewTester(coal.MustCreateStore("mongodb://0.0.0.0:27017/test-flame"), &User{}, &Application{}, &AccessToken{}, &RefreshToken{})

func newHandler(auth *Authenticator, force bool) http.Handler {
	router := http.NewServeMux()
	router.Handle("/oauth2/", auth.Endpoint("/oauth2/"))

	authorizer := auth.Authorizer("foo", force, true, true)
	router.Handle("/api/protected", authorizer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("OK"))
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

func TestMain(m *testing.M) {
	tr := transport.NewHTTPTransport("http://0.0.0.0:14268/api/traces?format=jaeger.thrift")

	tracer, closer := jaeger.NewTracer("test-flame",
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(tr),
	)

	opentracing.SetGlobalTracer(tracer)

	m.Run()

	_ = closer.Close()
	_ = tr.Close()
}
