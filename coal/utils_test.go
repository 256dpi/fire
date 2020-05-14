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

func (m *postModel) Validate() error {
	return nil
}

type commentModel struct {
	Base    `json:"-" bson:",inline" coal:"comments"`
	Message string `json:"message"`
	Parent  *ID    `json:"-" coal:"parent:comments"`
	Post    ID     `json:"-" bson:"post_id" coal:"post:posts"`
}

func (m *commentModel) Validate() error {
	return nil
}

type selectionModel struct {
	Base  `json:"-" bson:",inline" coal:"selections:selections"`
	Name  string `json:"name"`
	Posts []ID   `json:"-" bson:"post_ids" coal:"posts:posts"`
}

func (m *selectionModel) Validate() error {
	return nil
}

type noteModel struct {
	Base      `json:"-" bson:",inline" coal:"notes"`
	Title     string    `json:"title" bson:"title"`
	CreatedAt time.Time `json:"created-at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated-at" bson:"updated_at"`
	Post      ID        `json:"-" bson:"post_id" coal:"post:posts"`
}

func (m *noteModel) Validate() error {
	return nil
}

type polyModel struct {
	Base `json:"-" bson:",inline" coal:"polys"`
	Ref1 Ref   `json:"-" coal:"ref1:*"`
	Ref2 *Ref  `json:"-" coal:"ref2:posts"`
	Ref3 []Ref `json:"-" coal:"ref3:notes+selections"`
}

func (m *polyModel) Validate() error {
	return nil
}

var mongoStore = MustConnect("mongodb://0.0.0.0/test-fire-coal")
var lungoStore = MustOpen(nil, "test-fire-coal", nil)

var modelList = []Model{&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{}, &polyModel{}}

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
