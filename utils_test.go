package fire

import (
	"errors"

	"github.com/256dpi/fire/coal"

	"gopkg.in/mgo.v2/bson"
)

type postModel struct {
	coal.Base  `json:"-" bson:",inline" valid:"required" coal:"posts"`
	Title      string       `json:"title" bson:"title" valid:"required"`
	Published  bool         `json:"published"`
	TextBody   string       `json:"text-body" bson:"text_body"`
	Comments   coal.HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	Selections coal.HasMany `json:"-" bson:"-" coal:"selections:selections:posts"`
	Note       coal.HasOne  `json:"-" bson:"-" coal:"note:notes:post"`
}

func (p *postModel) Validate() error {
	if p.Title == "error" {
		return errors.New("error")
	}

	return nil
}

type commentModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"comments"`
	Message   string         `json:"message"`
	Parent    *bson.ObjectId `json:"-" bson:"parent_id" valid:"object-id" coal:"parent:comments"`
	Post      bson.ObjectId  `json:"-" bson:"post_id" valid:"required,object-id" coal:"post:posts"`
}

type selectionModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"selections:selections"`
	Name      string          `json:"name"`
	Posts     []bson.ObjectId `json:"-" bson:"post_ids" valid:"object-id" coal:"posts:posts"`
}

type noteModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"notes"`
	Title     string        `json:"title" bson:"title" valid:"required"`
	Post      bson.ObjectId `json:"-" bson:"post_id" valid:"required,object-id" coal:"post:posts"`
}

type fooModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"foos"`
	Foo       bson.ObjectId   `json:"-" bson:"foo_id" valid:"required,object-id" coal:"foo:foos"`
	OptFoo    *bson.ObjectId  `json:"-" bson:"opt_foo_id" valid:"object-id" coal:"foo:foos"`
	Foos      []bson.ObjectId `json:"-" bson:"foo_ids" valid:"object-id" coal:"foo:foos"`
	Bar       bson.ObjectId   `json:"-" bson:"bar_id" valid:"required,object-id" coal:"bar:bars"`
	OptBar    *bson.ObjectId  `json:"-" bson:"opt_bar_id" valid:"object-id" coal:"bar:bars"`
	Bars      []bson.ObjectId `json:"-" bson:"bar_ids" valid:"object-id" coal:"bar:bars"`
}

type barModel struct {
	coal.Base `json:"-" bson:",inline" valid:"required" coal:"bars"`
	Foo       bson.ObjectId `json:"-" bson:"foo_id" valid:"required,object-id" coal:"foo:foos"`
}

var testStore = coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire")
var testSubStore = testStore.Copy()

var tester = NewTester(testStore, &postModel{}, &commentModel{}, &selectionModel{}, &noteModel{}, &fooModel{}, &barModel{})
