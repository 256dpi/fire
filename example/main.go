package main

import (
	"net/http"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/auth"
	"golang.org/x/crypto/bcrypt"
)

type post struct {
	fire.Base `json:"-" bson:",inline" fire:"posts"`
	Slug      string `json:"slug" valid:"required" bson:"slug"`
	Title     string `json:"title" valid:"required"`
	Body      string `json:"body" valid:"-"`
}

type user struct {
	fire.Base    `json:"-" bson:",inline" fire:"users"`
	Email        string `json:"email" valid:"required,email"`
	FullName     string `json:"full-name" valid:"required"`
	PasswordHash []byte `json:"-" valid:"required"`
	Admin        bool   `json:"admin" valid:"required"`
}

func (u *user) ResourceOwnerIdentifier() string {
	return "Email"
}

func (u *user) ValidPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)) == nil
}

const secret = "abcd1234abcd1234"

func main() {
	// create store
	store := fire.MustCreateStore("mongodb://localhost/fire-example")

	// clean resources
	store.DB().C("applications").RemoveAll(nil)
	store.DB().C("users").RemoveAll(nil)
	store.DB().C("posts").RemoveAll(nil)
	store.DB().C("access_tokens").RemoveAll(nil)

	// create policy
	policy := auth.DefaultPolicy([]byte(secret))

	// enable OAuth2 password grant
	policy.PasswordGrant = true

	// provide custom user model
	policy.ResourceOwner = &user{}

	// create authenticator
	a11r := auth.New(store, policy)

	// pre hash the password
	password, err := bcrypt.GenerateFromPassword([]byte("abcd1234"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	// create test client
	client := fire.Init(&auth.Application{
		Name:        "test",
		Key:         "abcd1234",
		SecretHash:  password,
		Scope:       "default admin",
		RedirectURI: "https://0.0.0.0:8080/auth/callback",
	})

	// save test client
	err = store.C(client).Insert(client)
	if err != nil {
		panic(err)
	}

	// create test user
	owner := fire.Init(&user{
		FullName:     "Test User",
		Email:        "test@example.com",
		PasswordHash: password,
	})

	// create admin user
	admin := fire.Init(&user{
		FullName:     "Admin User",
		Email:        "admin@example.com",
		PasswordHash: password,
		Admin:        true,
	})

	// save users
	err = store.C(&user{}).Insert(owner, admin)
	if err != nil {
		panic(err)
	}

	// create group
	group := fire.NewGroup()

	// register post controller
	group.Add(&fire.Controller{
		Model: &post{},
		Store: store,
		FilterableFields: []string{
			"slug",
		},
		SortableFields: []string{
			"slug",
		},
		Authorizer: auth.Callback("default"),
		Validator:  fire.ModelValidator(),
	}, &fire.Controller{
		Model:      &user{},
		Store:      store,
		Authorizer: auth.Callback("default admin"),
		Validator:  fire.ModelValidator(),
	}, &fire.Controller{
		Model:      &auth.Application{},
		Store:      store,
		Authorizer: auth.Callback("default admin"),
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
