package fire

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/256dpi/fire/coal"

	"gopkg.in/mgo.v2/bson"
)

type postModel struct {
	coal.Base  `json:"-" bson:",inline" coal:"posts"`
	Title      string       `json:"title" bson:"title" valid:"required"`
	Published  bool         `json:"published"`
	TextBody   string       `json:"text-body" bson:"text_body"`
	Comments   coal.HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	Selections coal.HasMany `json:"-" bson:"-" coal:"selections:selections:posts"`
}

func (p *postModel) Validate() error {
	if p.Title == "error" {
		return errors.New("error")
	}

	return nil
}

type commentModel struct {
	coal.Base `json:"-" bson:",inline" coal:"comments"`
	Message   string         `json:"message"`
	Parent    *bson.ObjectId `json:"-" coal:"parent:comments"`
	Post      bson.ObjectId  `json:"-" bson:"post_id" coal:"post:posts"`
}

type selectionModel struct {
	coal.Base `json:"-" bson:",inline" coal:"selections:selections"`
	Name      string          `json:"name"`
	Posts     []bson.ObjectId `json:"-" bson:"post_ids" coal:"posts:posts"`
}

var testStore = coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire")
var testSubStore = testStore.Copy()

func cleanStore() {
	testSubStore.DB().C("posts").RemoveAll(nil)
	testSubStore.DB().C("comments").RemoveAll(nil)
	testSubStore.DB().C("selections").RemoveAll(nil)
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

func saveModel(m coal.Model) coal.Model {
	err := testSubStore.C(m).Insert(m)
	if err != nil {
		panic(err)
	}

	return m
}

func findLastModel(m coal.Model) coal.Model {
	err := testSubStore.C(m).Find(nil).Sort("-_id").One(m)
	if err != nil {
		panic(err)
	}

	return coal.Init(m)
}
