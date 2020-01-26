package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/serve"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/spark"
)

var port = getEnv("PORT", "8000")
var mongoURI = getEnv("MONGODB_URI", "")
var secret = getEnv("SECRET", "abcd1234abcd1234")
var mainKey = getEnv("MAIN_KEY", "main-key")
var subKey = getEnv("SUB_KEY", "sub-key")

func main() {
	// visualize as PDF
	pdf, err := catalog.VisualizePDF("Example")
	if err != nil {
		panic(err)
	}

	// write visualization dot
	err = ioutil.WriteFile("models.pdf", pdf, 0777)
	if err != nil {
		panic(err)
	}

	// create store
	var store *coal.Store
	if mongoURI != "" {
		store = coal.MustConnect(mongoURI)
	} else {
		store = coal.MustOpen(nil, "example", func(err error) {
			panic(err)
		})
	}

	// prepare database
	err = prepareDatabase(store)
	if err != nil {
		panic(err)
	}

	// get port
	port, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	// check secret
	if len(secret) < 16 {
		panic("secret must be at least 16 characters")
	}

	// prepare cors options
	corsOptions := serve.CORSPolicy{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Origin", "Accept", "Content-Type",
			"Authorization", "Cache-Control", "X-Requested-With"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE"},
	}

	// compose handler
	handler := serve.Compose(
		flame.TokenMigrator(true),
		fire.RootTracer(),
		serve.CORS(corsOptions),
		createHandler(store),
	)

	// configure jaeger tracer
	configureJaeger()

	// run http server
	fmt.Printf("Running on http://0.0.0.0:%d\n", port)
	err = http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), handler)
	if err != nil {
		panic(err)
	}
}

func prepareDatabase(store *coal.Store) error {
	// ensure indexes
	err := catalog.EnsureIndexes(store)
	if err != nil {
		return err
	}

	// ensure first user
	err = flame.EnsureFirstUser(store, "User", "user@example.org", "user1234")
	if err != nil {
		return err
	}

	// ensure main application
	mainKey, err = flame.EnsureApplication(store, "Main", mainKey, "")
	if err != nil {
		return err
	}

	// ensure sub application
	subKey, err = flame.EnsureApplication(store, "Sub", subKey, "", "http://0.0.0.0:4200/return")
	if err != nil {
		return err
	}

	// log keys
	fmt.Printf("Main Application Key: %s\n", mainKey)
	fmt.Printf("Sub Application Key: %s\n", subKey)

	return nil
}

func createHandler(store *coal.Store) http.Handler {
	// prepare master
	master := heat.Secret(secret)

	// derive secrets
	authSecret := master.Derive("auth")
	fileSecret := master.Derive("file")

	// create reporter
	reporter := func(err error) {
		println(err.Error())
	}

	// create mux
	mux := http.NewServeMux()

	// create policy
	policy := flame.DefaultPolicy(heat.NewNotary("example", authSecret))
	policy.Grants = flame.StaticGrants(true, true, true, true, true)
	policy.ApprovalURL = flame.StaticApprovalURL("http://0.0.0.0:4200/authorize")
	policy.GrantStrategy = func(client flame.Client, owner flame.ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error) {
		return scope, nil
	}
	policy.ApproveStrategy = func(client flame.Client, owner flame.ResourceOwner, token flame.GenericToken, scope oauth2.Scope) (oauth2.Scope, error) {
		return scope, nil
	}

	// create authenticator
	a := flame.NewAuthenticator(store, policy, reporter)

	// register authenticator
	mux.Handle("/auth/", a.Endpoint("/auth/"))

	// create watcher
	watcher := spark.NewWatcher(reporter)
	watcher.Add(itemStream(store))
	watcher.Add(jobStream(store))
	watcher.Add(valueStream(store))
	watcher.Add(fileStream(store))

	// create storage
	storage := &blaze.Storage{
		Store:   store,
		Notary:  heat.NewNotary("example", fileSecret),
		Service: blaze.NewGridFSService(store, serve.MustByteSize("1M")),
	}

	// create queue
	queue := axe.NewQueue(store, reporter)
	queue.Add(incrementTask())
	queue.Add(periodicTask())
	queue.Add(storage.CleanupTask(time.Minute, time.Minute))
	queue.Run()

	// create group
	g := fire.NewGroup(reporter)

	// add controllers
	g.Add(itemController(store, queue, storage))
	g.Add(userController(store))
	g.Add(jobController(store))
	g.Add(valueController(store))
	g.Add(fileController(store))

	// add watch action
	g.Handle("watch", &fire.GroupAction{
		Authorizers: fire.L{
			flame.Callback(true),
		},
		Action: watcher.Action(),
	})

	// add upload action
	g.Handle("upload", &fire.GroupAction{
		Authorizers: fire.L{
			flame.Callback(true),
		},
		Action: storage.Upload(serve.MustByteSize("16M")),
	})

	// add download action
	g.Handle("download", &fire.GroupAction{
		Authorizers: fire.L{
			// public endpoint
		},
		Action: storage.Download(),
	})

	// register group
	mux.Handle("/api/", serve.Compose(
		a.Authorizer("", false, true, true),
		g.Endpoint("/api/"),
	))

	return mux
}

func configureJaeger() {
	// create transport
	tr := transport.NewHTTPTransport("http://0.0.0.0:14268/api/traces?format=jaeger.thrift")

	// create tracer
	tracer, _ := jaeger.NewTracer("example",
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(tr),
	)

	// set global tracer
	opentracing.SetGlobalTracer(tracer)
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v != "" {
		return v
	}

	return def
}
