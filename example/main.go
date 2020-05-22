package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/256dpi/lungo"
	"github.com/256dpi/oauth2/v2"
	"github.com/256dpi/serve"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/cinder"
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

var reporter = func(err error) {
	println(err.Error())
}

var reporterWithStack = func(err error) {
	reporter(err)
	debug.PrintStack()
}

func main() {
	// visualize models
	err := catalog.Visualize("Example", "models.pdf")
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

	// create bucket
	bucket := lungo.NewBucket(store.DB())

	// prepare database
	err = prepareDatabase(store, bucket)
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
		serve.Recover(reporterWithStack),
		serve.Throttle(100),
		serve.Timeout(time.Minute),
		serve.Limit(serve.MustByteSize("8M")),
		serve.CORS(corsOptions),
		flame.TokenMigrator(true),
		cinder.RootHandler(),
		createHandler(store, bucket),
	)

	// configure local jaeger
	cinder.SetupTesting("example")

	// run http server
	fmt.Printf("Running on http://0.0.0.0:%d\n", port)
	err = http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), handler)
	if err != nil {
		panic(err)
	}
}

func prepareDatabase(store *coal.Store, bucket *lungo.Bucket) error {
	// ensure indexes
	err := catalog.EnsureIndexes(store)
	if err != nil {
		return err
	}

	// ensure bucket indexes
	err = bucket.EnsureIndexes(nil, false)
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

	// log info
	fmt.Printf("==> Password for user@example.org: user1234\n")
	fmt.Printf("==> Main application key: %s\n", mainKey)
	fmt.Printf("==> Sub application key: %s\n", subKey)

	return nil
}

func createHandler(store *coal.Store, bucket *lungo.Bucket) http.Handler {
	// prepare master secret
	masterSecret := heat.Secret(secret)

	// derive secrets
	authSecret := masterSecret.Derive("auth")
	fileSecret := masterSecret.Derive("file")

	// create mux
	mux := http.NewServeMux()

	// create policy
	policy := flame.DefaultPolicy(heat.NewNotary("example/auth", authSecret))
	policy.Grants = flame.StaticGrants(true, true, true, true, true)
	policy.ApprovalURL = flame.StaticApprovalURL("http://0.0.0.0:4200/authorize")
	policy.GrantStrategy = func(_ *flame.Context, client flame.Client, owner flame.ResourceOwner, scope oauth2.Scope) (oauth2.Scope, error) {
		return scope, nil
	}
	policy.ApproveStrategy = func(_ *flame.Context, client flame.Client, owner flame.ResourceOwner, token flame.GenericToken, scope oauth2.Scope) (oauth2.Scope, error) {
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
	fileNotary := heat.NewNotary("example/file", fileSecret)
	fileService := blaze.NewGridFS(bucket)
	storage := blaze.NewStorage(store, fileNotary, fileService, register)

	// create queue
	queue := axe.NewQueue(axe.Options{
		Store:    store,
		Reporter: reporter,
	})

	// add tasks
	queue.Add(incrementTask(store))
	queue.Add(generateTask(store, storage))
	queue.Add(periodicTask(store))
	queue.Add(storage.CleanupTask(time.Minute, time.Minute, time.Minute, time.Minute))

	// qun queue
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
		Action: storage.UploadAction(serve.MustByteSize("16M")),
	})

	// add download action
	g.Handle("download", &fire.GroupAction{
		Authorizers: fire.L{
			// public endpoint
		},
		Action: storage.DownloadAction(),
	})

	// register group
	mux.Handle("/api/", serve.Compose(
		a.Authorizer("", false, true, true),
		g.Endpoint("/api/"),
	))

	return mux
}

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v != "" {
		return v
	}

	return def
}
