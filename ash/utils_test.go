package ash

import (
	"errors"
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
)

var tester = fire.NewTester(coal.MustCreateStore("mongodb://localhost/test-ash"))

func blank() *Authorizer {
	return A("blank", nil, func(_ *fire.Context) ([]*Enforcer, error) {
		return nil, nil
	})
}

func accessGranted() *Authorizer {
	return A("accessGranted", nil, func(_ *fire.Context) ([]*Enforcer, error) {
		return S{GrantAccess()}, nil
	})
}

func accessDenied() *Authorizer {
	return A("accessDenied", nil, func(_ *fire.Context) ([]*Enforcer, error) {
		return S{DenyAccess()}, nil
	})
}

func directError() *Authorizer {
	return A("directError", nil, func(_ *fire.Context) ([]*Enforcer, error) {
		return nil, errors.New("error")
	})
}

func indirectError() *Authorizer {
	return A("indirectError", nil, func(_ *fire.Context) ([]*Enforcer, error) {
		return S{E("indirectError", nil, func(_ *fire.Context) error {
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

	m.Run()
}
