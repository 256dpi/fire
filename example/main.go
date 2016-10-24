package main

import (
	"os"

	"github.com/gonfire/fire"
	"github.com/gonfire/fire/components"
	"github.com/gonfire/fire/jsonapi"
	"github.com/gonfire/fire/model"
	"github.com/gonfire/fire/auth"
	"golang.org/x/crypto/bcrypt"
)

type post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Slug       string `json:"slug" valid:"required" bson:"slug"`
	Title      string `json:"title" valid:"required"`
	Body       string `json:"body" valid:"-"`
}

type user struct {
	model.Base   `json:"-" bson:",inline" fire:"users"`
	Email        string `json:"email" valid:"required,email"`
	FullName     string `json:"full-name" valid:"required"`
	PasswordHash []byte `json:"-" valid:"required"`
	Admin        bool   `json:"admin" valid:"required"`
}

func (u *user) OAuthIdentifier() string {
	return "Email"
}

func (u *user) GetOAuthData() []byte {
	return u.PasswordHash
}

func main() {
	// create store
	store := model.MustCreateStore("mongodb://localhost/fire-example")

	// clean resources
	store.DB().C("applications").RemoveAll(nil)
	store.DB().C("users").RemoveAll(nil)
	store.DB().C("posts").RemoveAll(nil)
	store.DB().C("access_tokens").RemoveAll(nil)

	// create policy
	policy := auth.DefaultPolicy([]byte(os.Getenv("SECRET")))

	// enable OAuth2 password grant
	policy.PasswordGrant = true

	// provide custom user model
	policy.ResourceOwner = &user{}

	// provide custom grant strategy
	policy.GrantStrategy = func(req *auth.GrantRequest) []string {
		list := []string{"default"}

		// always grant the admin scope to admins
		if user, ok := req.ResourceOwner.(*user); ok && user.Admin {
			list = append(list, "admin")
		}

		return list
	}

	// create authenticator
	authenticator := auth.New(store, policy, "auth")

	// pre hash the password
	password, err := bcrypt.GenerateFromPassword([]byte("abcd1234"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	// create test client
	client := model.Init(&auth.Application{
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
	owner := model.Init(&user{
		FullName:     "Test User",
		Email:        "test@example.com",
		PasswordHash: password,
	})

	// create admin user
	admin := model.Init(&user{
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
	group := jsonapi.NewGroup("api")

	// register post controller
	group.Add(&jsonapi.Controller{
		Model: &post{},
		Store: store,
		FilterableFields: []string{
			"slug",
		},
		SortableFields: []string{
			"slug",
		},
		Authorizer: authenticator.Authorizer("default"),
		Validator:  jsonapi.ModelValidator(),
	}, &jsonapi.Controller{
		Model:      &user{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default admin"),
		Validator:  jsonapi.ModelValidator(),
	}, &jsonapi.Controller{
		Model:      &auth.Application{},
		Store:      store,
		Authorizer: authenticator.Authorizer("default admin"),
		Validator:  jsonapi.ModelValidator(),
	})

	// prepare app
	app := fire.New()

	// mount inspector
	app.Mount(fire.DefaultInspector())

	// mount protector
	app.Mount(components.DefaultProtector())

	// mount authenticator
	app.Mount(authenticator)

	// mount group
	app.Mount(group)

	// mount ember server
	app.Mount(components.DefaultAssetServer("./ui/dist"))

	// run app
	app.StartSecure("localhost:8080", "ssl/server.crt", "ssl/server.key")

	// yield app
	app.Yield()
}
