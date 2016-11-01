package fire

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	Base       `json:"-" bson:",inline" fire:"posts"`
	Title      string  `json:"title" bson:"title" valid:"required"`
	Published  bool    `json:"published"`
	TextBody   string  `json:"text-body" bson:"text_body"`
	Comments   HasMany `json:"-" bson:"-" fire:"comments:comments:post"`
	Selections HasMany `json:"-" bson:"-" fire:"selections:selections:posts"`
}

func (p *Post) Validate() error {
	if p.Title == "error" {
		return errors.New("error")
	}

	return nil
}

type Comment struct {
	Base    `json:"-" bson:",inline" fire:"comments"`
	Message string         `json:"message"`
	Parent  *bson.ObjectId `json:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	Base    `json:"-" bson:",inline" fire:"selections:selections"`
	Name    string          `json:"name"`
	PostIDs []bson.ObjectId `json:"-" bson:"post_ids" fire:"posts:posts"`
}

var testStore = MustCreateStore("mongodb://0.0.0.0:27017/fire")

func getCleanStore() *Store {
	testStore.DB().C("posts").RemoveAll(nil)
	testStore.DB().C("comments").RemoveAll(nil)
	testStore.DB().C("selections").RemoveAll(nil)

	return testStore
}

func buildHandler(controllers ...*Controller) http.Handler {
	group := NewGroup()
	group.Add(controllers...)
	return group.Endpoint("")
}

func testRequest(h http.Handler, method, path string, headers map[string]string, payload string, callback func(*httptest.ResponseRecorder, *http.Request)) {
	r, err := http.NewRequest(method, path, strings.NewReader(payload))
	if err != nil {
		panic(err)
	}

	w := httptest.NewRecorder()

	for k, v := range headers {
		r.Header.Set(k, v)
	}

	h.ServeHTTP(w, r)

	callback(w, r)
}

func saveModel(m Model) Model {
	err := testStore.C(m).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func findLastModel(m Model) Model {
	err := testStore.C(m).Find(nil).Sort("-_id").One(m)
	if err != nil {
		panic(err)
	}

	return Init(m)
}

func testReporter(t *testing.T) func(error) {
	return func(err error) {
		t.Error(err)
	}
}
