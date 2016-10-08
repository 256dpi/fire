package jsonapi

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Title      string        `json:"title" valid:"required" bson:"title"`
	Published  bool          `json:"published"`
	TextBody   string        `json:"text-body" bson:"text_body"`
	Comments   model.HasMany `json:"-" bson:"-" fire:"comments:comments:post"`
	Selections model.HasMany `json:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	model.Base `json:"-" bson:",inline" fire:"comments"`
	Message    string         `json:"message"`
	Parent     *bson.ObjectId `json:"-" fire:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	model.Base `json:"-" bson:",inline" fire:"selections:selections"`
	Name       string          `json:"name"`
	PostIDs    []bson.ObjectId `json:"-" bson:"post_ids" fire:"posts:posts"`
}

var testStore = model.MustCreateStore("mongodb://0.0.0.0:27017/fire")

func getCleanStore() *model.Store {
	testStore.DB().C("posts").RemoveAll(nil)
	testStore.DB().C("comments").RemoveAll(nil)
	testStore.DB().C("selections").RemoveAll(nil)

	return testStore
}

func buildServer(controllers ...*Controller) *echo.Echo {
	router := echo.New()

	group := NewGroup("")
	group.Add(controllers...)
	group.Register(router)

	return router
}

func testRequest(e *echo.Echo, method, path string, headers map[string]string, payload string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	r, err := http.NewRequest(method, path, strings.NewReader(payload))
	if err != nil {
		panic(err)
	}

	w := httptest.NewRecorder()

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	e.ServeHTTP(standard.NewRequest(r, nil), standard.NewResponse(w, nil))

	callback(w, r)
}

func saveModel(m model.Model) model.Model {
	err := testStore.C(m).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func findLastModel(m model.Model) model.Model {
	err := testStore.C(m).Find(nil).Sort("-_id").One(m)
	if err != nil {
		panic(err)
	}

	return model.Init(m)
}
