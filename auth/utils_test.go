package auth

import (
	"net/http"

	"github.com/gonfire/fire/model"
	"github.com/pressly/chi"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Title      string        `json:"title" valid:"required" bson:"title"`
	TextBody   string        `json:"text-body" valid:"-" bson:"text_body"`
	Comments   model.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
}

type Comment struct {
	model.Base `json:"-" bson:",inline" fire:"comments"`
	Message    string         `json:"message" valid:"required"`
	Parent     *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

var testStore = model.MustCreateStore("mongodb://0.0.0.0:27017/fire")

func getCleanStore() *model.Store {
	testStore.DB().C("posts").RemoveAll(nil)
	testStore.DB().C("comments").RemoveAll(nil)
	testStore.DB().C("selections").RemoveAll(nil)
	testStore.DB().C("users").RemoveAll(nil)
	testStore.DB().C("applications").RemoveAll(nil)
	testStore.DB().C("access_tokens").RemoveAll(nil)

	return testStore
}

func newHandler(auth *Authenticator) http.Handler {
	router := chi.NewRouter()

	auth.Register(nil, router)

	router.With(auth.Authorize("foo")).
		HandleFunc("/api/protected", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		})

	return router
}

func saveModel(m model.Model) model.Model {
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
