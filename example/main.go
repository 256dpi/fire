package main

import (
	"net/http"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/auth"
	"github.com/gonfire/fire/tools"
)

type post struct {
	fire.Base `json:"-" bson:",inline" fire:"posts"`
	Slug      string `json:"slug" valid:"required" bson:"slug"`
	Title     string `json:"title" valid:"required"`
	Body      string `json:"body" valid:"-"`
}

func main() {
	// create store
	store := fire.MustCreateStore("mongodb://localhost/fire-example")

	// create policy
	policy := auth.DefaultPolicy("abcd1234abcd1234")

	// enable OAuth2 password grant
	policy.PasswordGrant = true

	// create authenticator
	authenticator := auth.New(store, policy)

	// create group
	group := fire.NewGroup()

	// register post controller
	group.Add(&fire.Controller{
		Model:      &post{},
		Store:      store,
		Filters:    []string{"slug"},
		Sorters:    []string{"slug"},
		Authorizer: auth.Callback("default"),
		Validator:  fire.ModelValidator(),
	})

	// create new router
	router := http.NewServeMux()

	// create oauth2 and api endpoint
	authEndpoint := authenticator.Endpoint("/oauth2/")
	apiEndpoint := group.Endpoint("/api/")

	// create spa asset server
	spaEndpoint := tools.DefaultAssetServer("../.test/assets/")

	// create protector, logger
	protector := tools.DefaultProtector()
	logger := tools.DefaultRequestLogger()

	// create authorizer
	authorizer := authenticator.Authorizer("")

	// mount authenticator, controller group, asset server
	router.Handle("/oauth2/", fire.Compose(authEndpoint, protector, logger))
	router.Handle("/api/", fire.Compose(apiEndpoint, protector, logger, authorizer))
	router.Handle("/", fire.Compose(spaEndpoint, protector, logger))

	// run app
	http.ListenAndServe("localhost:8080", router)
}
