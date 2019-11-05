package coal

import (
	"testing"
	"time"
)

type postModel struct {
	Base       `json:"-" bson:",inline" coal:"posts"`
	Title      string  `json:"title" bson:"title" coal:"foo"`
	Published  bool    `json:"published" coal:"bar"`
	TextBody   string  `json:"text-body" bson:"text_body" coal:"bar,baz"`
	Comments   HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	Selections HasMany `json:"-" bson:"-" coal:"selections:selections:posts"`
	Note       HasOne  `json:"-" bson:"-" coal:"note:notes:post"`
}

type commentModel struct {
	Base    `json:"-" bson:",inline" coal:"comments"`
	Message string `json:"message"`
	Parent  *ID    `json:"-" coal:"parent:comments"`
	Post    ID     `json:"-" bson:"post_id" coal:"post:posts"`
}

type selectionModel struct {
	Base  `json:"-" bson:",inline" coal:"selections:selections"`
	Name  string `json:"name"`
	Posts []ID   `json:"-" bson:"post_ids" coal:"posts:posts"`
}

type noteModel struct {
	Base      `json:"-" bson:",inline" coal:"notes"`
	Title     string    `json:"title" bson:"title"`
	CreatedAt time.Time `json:"created-at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated-at" bson:"updated_at"`
	Post      ID        `json:"-" bson:"post_id" coal:"post:posts"`
}

var mongoStore = MustCreateStore("mongodb://0.0.0.0/test-fire-coal")
var lungoStore = MustCreateStore("memory://test-fire-coal")

var modelList = []Model{&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{}}

func withTester(t *testing.T, fn func(*testing.T, *Tester)) {
	t.Run("Mongo", func(t *testing.T) {
		tester := NewTester(mongoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})

	t.Run("Lungo", func(t *testing.T) {
		tester := NewTester(lungoStore, modelList...)
		tester.Clean()
		fn(t, tester)
	})
}
