package cinder

import (
	"flag"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
)

// SetupTesting will configure a local jaeger instance to receive test traces.
func SetupTesting(name string) func() {
	// skip if benchmark
	if isBenchmark() {
		println("benchmark detected")
		return func() {}
	}

	// create transport
	tr := transport.NewHTTPTransport("http://0.0.0.0:14268/api/traces?format=jaeger.thrift")

	// create tracer
	tracer, closer := jaeger.NewTracer(name,
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(tr),
	)

	// set global tracer
	opentracing.SetGlobalTracer(tracer)

	return func() {
		_ = closer.Close()
		_ = tr.Close()
	}
}

func isBenchmark() bool {
	// check bench flag
	f := flag.Lookup("test.bench")
	if f != nil && f.Value.String() != "" {
		return true
	}

	return false
}
