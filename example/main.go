package main

import (
	"net/http"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/auth"
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
	a11r := auth.New(store, policy)

	// create group
	group := fire.NewGroup()

	// register post controller
	group.Add(&fire.Controller{
		Model: &post{},
		Store: store,
		FilterableFields: []string{"slug"},
		SortableFields: []string{"slug"},
		Authorizer: auth.Callback("default"),
		Validator:  fire.ModelValidator(),
	})

	// create new router
	router := http.NewServeMux()

	// create oauth2 and api endpoint
	oauth2 := a11r.Endpoint("/oauth2/")
	api := group.Endpoint("/api/")

	// create asset server
	assetServer := fire.DefaultAssetServer("../.test/assets/")

	// create protector, logger
	p := fire.DefaultProtector()
	l := fire.DefaultRequestLogger()

	// create authorizer
	a := a11r.Authorizer("")

	// mount authenticator, controller group, asset server
	router.Handle("/oauth2/", p(l(oauth2)))
	router.Handle("/api/", p(l(a(api))))
	router.Handle("/", p(l(assetServer)))

	// run app
	http.ListenAndServe("localhost:8080", router)
}
