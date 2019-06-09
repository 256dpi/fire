package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/goware/cors"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/flame"
	"github.com/256dpi/fire/spark"
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

	// prepare cors options
	corsOptions := cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedHeaders: []string{"Origin", "Accept", "Content-Type",
			"Authorization", "Cache-Control", "X-Requested-With"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE"},
	}

	// compose handler
	handler := fire.Compose(
		flame.TokenMigrator(true),
		fire.RootTracer(),
		cors.New(corsOptions).Handler,
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
	reporter := fire.ErrorReporter(os.Stderr)

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
	mux.Handle("/auth/", a.Endpoint("/auth/"))

	// create watcher
	watcher := spark.NewWatcher()
	watcher.Reporter = reporter
	watcher.Add(itemStream(store))

	// create queue
	queue := axe.NewQueue(store)

	// create pool
	pool := axe.NewPool()
	pool.Reporter = reporter
	pool.Add(incrementTask(store, queue))
	pool.Run()

	// create group
	g := fire.NewGroup()
	g.Reporter = reporter
	g.Add(itemController(store, queue))
	g.Add(userController(store))
	g.Handle("watch", &fire.GroupAction{
		Action: watcher.Action(),
	})

	// register group
	mux.Handle("/api/", fire.Compose(
		a.Authorizer("", true, true, true),
		g.Endpoint("/api/"),
	))

	return mux
}

func itemController(store *coal.Store, queue *axe.Queue) *fire.Controller {
	return &fire.Controller{
		Model: &Item{},
		Store: store,
		Validators: fire.L{
			// add basic validators
			fire.TimestampValidator(),
			fire.ModelValidator(),
			fire.RelationshipValidator(&Item{}, catalog),
		},
		ResourceActions: fire.M{
			"add": &fire.Action{
				Methods: []string{"POST"},
				Callback: queue.Callback("increment", 0, fire.All(), func(ctx *fire.Context) axe.Model {
					return &count{
						Item: ctx.Model.ID(),
					}
				}),
			},
		},
		UseTransactions:    true,
		TolerateViolations: true,
		IdempotentCreate:   true,
		ConsistentUpdate:   true,
		SoftDelete:         true,
	}
}

func userController(store *coal.Store) *fire.Controller {
	return &fire.Controller{
		Model: &flame.User{},
		Store: store,
		Validators: fire.L{
			// basic model & relationship validations
			fire.ModelValidator(),
			fire.RelationshipValidator(&flame.User{}, catalog),
		},
		TolerateViolations: true,
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

func incrementTask(store *coal.Store, queue *axe.Queue) *axe.Task {
	return &axe.Task{
		Name:  "increment",
		Queue: queue,
		Model: &count{},
		Handler: func(model axe.Model) (bson.M, error) {
			// get count
			c := model.(*count)

			// update document
			_, err := store.C(&Item{}).UpdateOne(nil, bson.M{
				"_id": c.Item,
			}, bson.M{
				"$inc": bson.M{
					coal.F(&Item{}, "Count"): 1,
				},
			})
			if err != nil {
				return nil, err
			}

			return nil, nil
		},
		Workers:     2,
		MaxAttempts: 2,
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
