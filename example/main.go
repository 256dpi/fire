package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
	"github.com/256dpi/fire/spark"
	"github.com/256dpi/fire/wood"

	"github.com/goware/cors"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
)

var port = getEnv("PORT", "8000")
var mongoURI = getEnv("MONGODB_URI", "mongodb://0.0.0.0/fire-example")
var secret = getEnv("SECRET", "abcd1234abcd1234")
var mainKey = getEnv("MAIN_KEY", "main-key")

func main() {
	// write visualization dot
	err := ioutil.WriteFile("models.dot", []byte(catalog.Visualize("Example")), 0777)
	if err != nil {
		panic(err)
	}

	// create store
	store, err := coal.CreateStore(mongoURI)
	if err != nil {
		panic(err)
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

	// create protector
	protector := wood.NewProtector("32M", cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Origin", "Accept", "Content-Type",
			"Authorization", "Cache-Control", "X-Requested-With"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE"},
	})

	// compose handler
	handler := fire.Compose(
		flame.TokenMigrator(true),
		fire.RootTracer(),
		protector,
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
	err := EnsureIndexes(store)
	if err != nil {
		return err
	}

	// ensure first user
	err = flame.EnsureFirstUser(store, "User", "user@example.org", "user1234")
	if err != nil {
		return err
	}

	// ensure main application
	mainKey, err = flame.EnsureApplication(store, "Main", mainKey, "1234abcd1234abcd")
	if err != nil {
		return err
	}

	// log key
	fmt.Printf("Main Application Key: %s\n", mainKey)

	return nil
}

func createHandler(store *coal.Store) http.Handler {
	// create reporter
	reporter := wood.DefaultErrorReporter()

	// create mux
	mux := http.NewServeMux()

	// create policy
	policy := flame.DefaultPolicy(secret)
	policy.PasswordGrant = true
	policy.ClientCredentialsGrant = true

	// create authenticator
	a := flame.NewAuthenticator(store, policy)
	a.Reporter = reporter

	// register authenticator
	mux.Handle("/v1/auth/", a.Endpoint("/v1/auth/"))

	// create watcher
	watcher := spark.NewWatcher()
	watcher.Reporter = reporter
	watcher.Add(itemStream(store))

	// create group
	g := fire.NewGroup()
	g.Reporter = reporter
	g.Add(itemController(store))
	g.Handle("watch", watcher.Action())

	// register group
	mux.Handle("/v1/api/", fire.Compose(
		a.Authorizer("", true, true, true),
		g.Endpoint("/v1/api/"),
	))

	return mux
}

func itemController(store *coal.Store) *fire.Controller {
	return &fire.Controller{
		Model: &Item{},
		Store: store,
		Validators: fire.L{
			// set timestamps
			fire.TimestampValidator("Created", ""),

			// basic model & relationship validations
			fire.ModelValidator(),
			fire.RelationshipValidator(&Item{}, catalog),
		},
		SoftProtection: true,
		SoftDelete:     true,
	}
}

func itemStream(store *coal.Store) *spark.Stream {
	return &spark.Stream{
		Model: &Item{},
		Store: store,
		Validator: func(sub *spark.Subscription) error {
			// check state
			if _, ok := sub.Data["state"].(bool); !ok {
				return fire.E("invalid state")
			}

			return nil
		},
		Selector: func(event *spark.Event, sub *spark.Subscription) bool {
			// check insert and update events
			return event.Model.(*Item).State == sub.Data["state"].(bool)
		},
		SoftDelete: true,
	}
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
