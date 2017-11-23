package fire

import (
	"errors"

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
	Note       coal.HasOne  `json:"-" bson:"-" coal:"note:notes:post"`
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
	Parent    *bson.ObjectId `json:"-" bson:"parent_id" coal:"parent:comments"`
	Post      bson.ObjectId  `json:"-" bson:"post_id" coal:"post:posts"`
}

type selectionModel struct {
	coal.Base `json:"-" bson:",inline" coal:"selections:selections"`
	Name      string          `json:"name"`
	Posts     []bson.ObjectId `json:"-" bson:"post_ids" coal:"posts:posts"`
}

type noteModel struct {
	coal.Base `json:"-" bson:",inline" coal:"notes"`
	Title     string        `json:"title" bson:"title" valid:"required"`
	Post      bson.ObjectId `json:"-" bson:"post_id" coal:"post:posts"`
}

var testStore = coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire")
var testSubStore = testStore.Copy()

var tester = NewTester(testStore, &postModel{}, &commentModel{}, &selectionModel{}, &noteModel{})
