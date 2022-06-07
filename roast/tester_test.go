package roast

import (
	"testing"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/coal"
)

func TestTester(t *testing.T) {
	tt := NewTester(Config{
		Models: catalog.All(),
	})

	tt.Assign("", &fire.Controller{
		Model: &fooModel{},
	})

	tt.List(t, &fooModel{}, nil)

	post := &fooModel{String: "String"}
	post = tt.Create(t, post, post, post).Model.(*fooModel)

	tt.List(t, &fooModel{}, []coal.Model{
		post,
	})

	post.Bool = true
	tt.Update(t, post, post, post)

	tt.Find(t, post, post)

	tt.Delete(t, post, nil)
}

func TestTesterErrors(t *testing.T) {
	tt := NewTester(Config{
		Models: catalog.All(),
	})

	tt.Assign("", &fire.Controller{
		Model: &fooModel{},
		Authorizers: fire.L{
			fire.C("Error", 0, fire.All(), func(ctx *fire.Context) error {
				if ctx.Operation == fire.Find || ctx.Operation == fire.List {
					return fire.ErrAccessDenied.Wrap()
				}
				return fire.ErrResourceNotFound.Wrap()
			}),
		},
	})

	post := tt.Insert(&fooModel{})

	tt.ListError(t, &fooModel{}, AccessDenied)
	tt.FindError(t, post, AccessDenied)
	tt.CreateError(t, post, ResourceNotFound)
	tt.UpdateError(t, post, ResourceNotFound)
	tt.DeleteError(t, post, ResourceNotFound)
}
