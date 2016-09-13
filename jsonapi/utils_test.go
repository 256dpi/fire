package jsonapi

import (
	"strings"

	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/test"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Title      string        `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published  bool          `json:"published" valid:"-" fire:"filterable"`
	TextBody   string        `json:"text-body" valid:"-" bson:"text_body"`
	Comments   model.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	Selections model.HasMany `json:"-" valid:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	model.Base `json:"-" bson:",inline" fire:"comments"`
	Message    string         `json:"message" valid:"required"`
	Parent     *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	model.Base `json:"-" bson:",inline" fire:"selections:selections"`
	Name       string          `json:"name" valid:"required"`
	PostIDs    []bson.ObjectId `json:"-" valid:"-" bson:"post_ids" fire:"posts:posts"`
}

var testStore = model.CreateStore("mongodb://0.0.0.0:27017/fire")

func getCleanStore() *model.Store {
	testStore.DB().C("posts").RemoveAll(nil)
	testStore.DB().C("comments").RemoveAll(nil)
	testStore.DB().C("selections").RemoveAll(nil)

	return testStore
}

func buildServer() *echo.Echo {
	store := getCleanStore()
	router := echo.New()
	group := New("")

	group.Add(&Controller{
		Model: &Post{},
		Store: store,
	}, &Controller{
		Model: &Comment{},
		Store: store,
	}, &Controller{
		Model: &Selection{},
		Store: store,
	})

	group.Register(router)

	return router
}

func testRequest(e *echo.Echo, method, path string, headers map[string]string, payload string, callback func(*test.ResponseRecorder, engine.Request)) {
	req := test.NewRequest(method, path, strings.NewReader(payload))
	rec := test.NewResponseRecorder()

	for k, v := range headers {
		req.Header().Set(k, v)
	}

	e.ServeHTTP(req, rec)

	callback(rec, req)
}

func saveModel(m model.Model) model.Model {
	model.Init(m)

	err := testStore.C(m).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func findLastModel(m model.Model) model.Model {
	model.Init(m)

	err := testStore.C(m).Find(nil).Sort("-_id").One(m)
	if err != nil {
		panic(err)
	}

	return m
}
