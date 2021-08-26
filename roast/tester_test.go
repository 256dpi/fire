package roast

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

type postModel struct {
	coal.Base `json:"-" bson:",inline" coal:"posts"`
	Title     string `json:"title"`
	Published bool   `json:"published"`

	stick.NoValidation `json:"-" bson:"-"`
}

var catalog = coal.NewCatalog(&postModel{})

func TestTester(t *testing.T) {
	tt := NewTester(Config{
		Models: catalog.Models(),
	})

	tt.Assign("", &fire.Controller{
		Model: &postModel{},
	})

	tt.List(t, &postModel{}, nil)

	post := &postModel{Title: "Hello"}
	post = tt.Create(t, post, post, post).Model.(*postModel)

	tt.List(t, &postModel{}, []coal.Model{
		post,
	})

	post.Published = true
	tt.Update(t, post, post, post)

	tt.Find(t, post, post)

	tt.Delete(t, post, nil)
}

func TestTesterErrors(t *testing.T) {
	tt := NewTester(Config{
		Models: catalog.Models(),
	})

	tt.Assign("", &fire.Controller{
		Model: &postModel{},
		Authorizers: fire.L{
			fire.C("Error", 0, fire.All(), func(ctx *fire.Context) error {
				if ctx.Operation == fire.Find || ctx.Operation == fire.List {
					return fire.ErrAccessDenied.Wrap()
				}
				return fire.ErrResourceNotFound.Wrap()
			}),
		},
	})

	post := tt.Insert(&postModel{})

	tt.ListError(t, &postModel{}, AccessDenied)
	tt.FindError(t, post, AccessDenied)
	tt.CreateError(t, post, ResourceNotFound)
	tt.UpdateError(t, post, ResourceNotFound)
	tt.DeleteError(t, post, ResourceNotFound)
}
