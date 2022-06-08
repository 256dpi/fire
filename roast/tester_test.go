package roast

import (
	"testing"

	"github.com/256dpi/lungo"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
)

func TestTester(t *testing.T) {
	tt := NewTester(Config{
		Models: models.All(),
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
		Models: models.All(),
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

func TestTesterUploadDownload(t *testing.T) {
	tt := NewTester(Config{
		DataNamespace:    "api",
		UploadEndpoint:   "upload",
		DownloadEndpoint: "download",
	})

	bucket := blaze.NewBucket(tt.Store, heat.NewNotary("foo", heat.MustRand(16)), &blaze.Binding{
		Name:  "foo",
		Model: &fooModel{},
		Field: "Link",
	})
	bucket.Use(blaze.NewGridFS(lungo.NewBucket(tt.Store.DB())), "local", true)

	group := fire.NewGroup(xo.Panic)
	group.Add(&fire.Controller{
		Model:      &fooModel{},
		Store:      tt.Store,
		Modifiers:  fire.L{bucket.Modifier()},
		Decorators: fire.L{bucket.Decorator()},
	})
	group.Handle("upload", &fire.GroupAction{
		Action: bucket.UploadAction(0),
	})
	group.Handle("download", &fire.GroupAction{
		Action: bucket.DownloadAction(),
	})

	tt.Handler = group.Endpoint("api")

	data := []byte("Hello World!")

	key := tt.Upload(t, data, "text/plain", "foo.txt")
	assert.NotZero(t, key)

	model := tt.Create(t, &fooModel{
		Link: &blaze.Link{
			ClaimKey: key,
		},
	}, nil, nil).Model.(*fooModel)

	buf := tt.Download(t, model.Link.ViewKey, "text/plain", "foo.txt", data)
	assert.Equal(t, data, buf)
}
