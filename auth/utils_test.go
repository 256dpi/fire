package auth

import (
	"net/http"
	"time"

	"github.com/gonfire/fire/model"
	"github.com/gonfire/oauth2"
	"github.com/gonfire/oauth2/bearer"
	"github.com/gonfire/oauth2/hmacsha"
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
	requiredScope := oauth2.ParseScope("foo")

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/token", auth.TokenEndpoint)
	mux.HandleFunc("/oauth2/authorize", auth.AuthorizationEndpoint)
	mux.HandleFunc("/api/protected", func(w http.ResponseWriter, r *http.Request) {
		// parse bearer token
		tk, res := bearer.ParseToken(r)
		if res != nil {
			bearer.WriteError(w, res)
			return
		}

		// parse token
		token, err := hmacsha.Parse(auth.Policy.Secret, tk)
		if err != nil {
			bearer.WriteError(w, bearer.InvalidToken("Malformed token"))
			return
		}

		// get token
		accessToken, err := auth.Storage.GetAccessToken(token.SignatureString())
		if err != nil {
			bearer.WriteError(w, err)
			return
		} else if accessToken == nil {
			bearer.WriteError(w, bearer.InvalidToken("Unkown token"))
			return
		}

		// get additional data
		data := accessToken.GetTokenData()

		// validate expiration
		if data.ExpiresAt.Before(time.Now()) {
			bearer.WriteError(w, bearer.InvalidToken("Expired token"))
			return
		}

		// validate scope
		if !data.Scope.Includes(requiredScope) {
			bearer.WriteError(w, bearer.InsufficientScope(requiredScope.String()))
			return
		}

		w.Write([]byte("OK"))
	})
	return mux
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
