package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/serve"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
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
	// create reporter
	reporter := func(err error) {
		println(err.Error())
	}

	// create mux
	mux := http.NewServeMux()

	// create policy
	policy := flame.DefaultPolicy(heat.NewNotary("example", heat.MustRand(32)))
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

	// create queue
	queue := axe.NewQueue(store, reporter)
	queue.Add(incrementTask())
	queue.Run()

	// create group
	g := fire.NewGroup(reporter)
	g.Add(itemController(store, queue))
	g.Add(userController(store))
	g.Handle("watch", &fire.GroupAction{
		Action: watcher.Action(),
	})

	// register group
	mux.Handle("/api/", serve.Compose(
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
			"add": queue.Action([]string{"POST"}, func(ctx *fire.Context) axe.Blueprint {
				return axe.Blueprint{
					Name: "increment",
					Model: &count{
						Item: ctx.Model.ID(),
					},
				}
			}),
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

func incrementTask() *axe.Task {
	return &axe.Task{
		Name:  "increment",
		Model: &count{},
		Handler: func(ctx *axe.Context) error {
			// get count
			c := ctx.Model.(*count)

			// update document
			_, err := ctx.TC(&Item{}).UpdateOne(ctx, bson.M{
				"_id": c.Item,
			}, bson.M{
				"$inc": bson.M{
					coal.F(&Item{}, "Count"): 1,
				},
			})
			if err != nil {
				return err
			}

			return nil
		},
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
