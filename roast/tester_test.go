package roast

import (
	"net/http"
	"testing"
	"time"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/lungo"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"

	"github.com/256dpi/fire"
	"github.com/256dpi/fire/axe"
	"github.com/256dpi/fire/blaze"
	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/heat"
	"github.com/256dpi/fire/stick"
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

func TestTesterCall(t *testing.T) {
	tt := NewTester(Config{
		Models: models.All(),
	})

	tt.Assign("", &fire.Controller{
		Model: &fooModel{},
		CollectionActions: fire.M{
			"bar": fire.A("foo", []string{"POST"}, 128, 0, func(ctx *fire.Context) error {
				return ctx.Respond(stick.Map{
					"ok": true,
				})
			}),
			"baz": fire.A("foo", []string{"POST"}, 128, 0, func(ctx *fire.Context) error {
				return xo.SF("failed")
			}),
		},
	})

	var out stick.Map
	code, err := tt.Call(t, tt.URL("foos", "bar"), stick.Map{"foo": "bar"}, &out)
	assert.Equal(t, http.StatusOK, code)
	assert.Nil(t, err)

	code, err = tt.Call(t, tt.URL("foos", "baz"), stick.Map{"foo": "bar"}, nil)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, jsonapi.BadRequest("failed"), err)
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

func TestTesterAwait(t *testing.T) {
	tt := NewTester(Config{
		Models: models.All(),
	})

	queue := axe.NewQueue(axe.Options{
		Store:    tt.Store,
		Reporter: xo.Panic,
	})
	queue.Add(&axe.Task{
		Job: &fooJob{},
		Handler: func(ctx *axe.Context) error {
			return nil
		},
	})
	queue.Run()

	n := tt.Await(t, 0, func() {
		ok, err := queue.Enqueue(nil, &fooJob{}, 0, 0)
		assert.NoError(t, err)
		assert.True(t, ok)
	})
	assert.Equal(t, 1, n)

	n = tt.Await(t, 10*time.Millisecond, func() {})
	assert.Equal(t, 0, n)
}
