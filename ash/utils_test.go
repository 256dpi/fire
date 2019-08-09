package ash

import (
	"errors"
	"os"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

var tester = fire.NewTester(coal.MustCreateStore("mongodb://0.0.0.0/test-fire-ash"))

func blank() *Authorizer {
	return A("blank", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return nil, nil
	})
}

func accessGranted() *Authorizer {
	return A("accessGranted", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{GrantAccess()}, nil
	})
}

func accessDenied() *Authorizer {
	return A("accessDenied", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{DenyAccess()}, nil
	})
}

func directError() *Authorizer {
	return A("directError", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return nil, errors.New("error")
	})
}

func indirectError() *Authorizer {
	return A("indirectError", fire.All(), func(_ *fire.Context) ([]*Enforcer, error) {
		return S{E("indirectError", fire.All(), func(_ *fire.Context) error {
			return errors.New("error")
		})}, nil
	})
}

func TestMain(m *testing.M) {
	tr := transport.NewHTTPTransport("http://0.0.0.0:14268/api/traces?format=jaeger.thrift")
	defer tr.Close()

	tracer, closer := jaeger.NewTracer("test-ash",
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(tr),
	)
	defer closer.Close()

	opentracing.SetGlobalTracer(tracer)

	os.Exit(m.Run())
}
