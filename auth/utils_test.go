package auth

import (
	"net/http"

	"github.com/gonfire/fire"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	fire.Base `json:"-" bson:",inline" fire:"posts"`
	Title     string       `json:"title" valid:"required" bson:"title"`
	TextBody  string       `json:"text-body" valid:"-" bson:"text_body"`
	Comments  fire.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
}

type Comment struct {
	fire.Base `json:"-" bson:",inline" fire:"comments"`
	Message   string         `json:"message" valid:"required"`
	Parent    *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID    bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

var testStore = fire.MustCreateStore("mongodb://0.0.0.0:27017/fire")

func getCleanStore() *fire.Store {
	testStore.DB().C("posts").RemoveAll(nil)
	testStore.DB().C("comments").RemoveAll(nil)
	testStore.DB().C("selections").RemoveAll(nil)
	testStore.DB().C("users").RemoveAll(nil)
	testStore.DB().C("applications").RemoveAll(nil)
	testStore.DB().C("access_tokens").RemoveAll(nil)

	return testStore
}

func newHandler(auth *Authenticator) http.Handler {
	router := http.NewServeMux()

	router.Handle("/oauth2/", auth.Endpoint("/oauth2/"))

	authorizer := auth.Authorizer("foo")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	router.Handle("/api/protected", authorizer(handler))

	return router
}

func saveModel(m fire.Model) fire.Model {
	err := testStore.C(m).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func mustHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	return hash
}
