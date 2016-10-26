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
	authenticator := auth.New(store, policy, "/oauth2/")

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
		Authorizer: authenticator.Authorizer("default"),
		Validator:  fire.ModelValidator(),
	}, &fire.Controller{
		Model:      &user{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default admin"),
		Validator:  fire.ModelValidator(),
	}, &fire.Controller{
		Model:      &auth.Application{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default admin"),
		Validator:  fire.ModelValidator(),
	})

	// create new router
	router := http.NewServeMux()

	// mount protector
	//app.Mount(components.DefaultProtector())

	// create api endpoint
	api := group.Endpoint("/api/")

	// create asset server
	assetServer := fire.DefaultAssetServer("../.test/assets/")

	// get request logger
	logger := fire.DefaultRequestLogger()

	// create authorizer
	authorizer := authenticator.Authorize("")

	// mount authenticator
	router.Handle("/oauth2/", logger(authenticator))

	// mount controller group
	router.Handle("/api/", logger(authorizer(api)))

	// mount ember server
	router.Handle("/", logger(assetServer))

	// run app
	http.ListenAndServe("localhost:8080", router)
}
