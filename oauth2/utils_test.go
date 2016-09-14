package oauth2

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/gonfire/fire/jsonapi"
	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Title      string        `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody   string        `json:"text-body" valid:"-" bson:"text_body"`
	Comments   model.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
}

type Comment struct {
	model.Base `json:"-" bson:",inline" fire:"comments"`
	Message    string         `json:"message" valid:"required"`
	Parent     *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

var session *mgo.Session

func init() {
	// connect to local mongodb
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire-auth")
	if err != nil {
		panic(err)
	}

	// store session globally
	session = sess
}

func getStore() *model.Store {
	return model.CreateStore("mongodb://0.0.0.0:27017/fire-auth")
}

func getDB() *mgo.Database {
	// get db
	db := session.DB("")

	// clean database by removing all documents
	db.C("posts").RemoveAll(nil)
	db.C("comments").RemoveAll(nil)
	db.C("users").RemoveAll(nil)
	db.C("applications").RemoveAll(nil)
	db.C("access_tokens").RemoveAll(nil)

	return db
}

func buildServer(controllers ...*jsonapi.Controller) (*echo.Echo, *mgo.Database) {
	db := getDB()
	router := echo.New()

	group := jsonapi.New("")
	group.Add(controllers...)
	group.Register(router)

	return router, db
}

func testRequest(e *echo.Echo, method, path string, headers map[string]string, form map[string]string, callback func(*httptest.ResponseRecorder, engine.Request)) {
	r, err := http.NewRequest(method, path, nil)
	if err != nil {
		panic(err)
	}

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	r.PostForm = make(url.Values)

	for k, v := range form {
		r.PostForm.Set(k, v)
	}

	rec := httptest.NewRecorder()

	req := standard.NewRequest(r, nil)
	res := standard.NewResponse(rec, nil)

	e.ServeHTTP(req, res)

	callback(rec, req)
}

func saveModel(db *mgo.Database, m model.Model) model.Model {
	model.Init(m)

	err := db.C(m.Meta().Collection).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func findModel(db *mgo.Database, m model.Model, query bson.M) model.Model {
	model.Init(m)

	err := db.C(m.Meta().Collection).Find(query).One(m)
	if err != nil {
		panic(err)
	}

	return m
}

func basicAuth(username, password string) map[string]string {
	auth := username + ":" + password
	auth = "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	return map[string]string{
		"Authorization": auth,
	}
}

func hashPassword(password string) []byte {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}

	return bytes
}
