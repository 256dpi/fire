package fire

import (
	"testing"
	"time"

	"github.com/256dpi/fire/coal"

	"github.com/globalsign/mgo/bson"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
)

type postModel struct {
	coal.Base  `json:"-" bson:",inline" coal:"posts"`
	Title      string       `json:"title" bson:"title"`
	Published  bool         `json:"published"`
	TextBody   string       `json:"text-body" bson:"text_body"`
	Deleted    *time.Time   `json:"-" bson:"deleted_at"`
	Comments   coal.HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	Selections coal.HasMany `json:"-" bson:"-" coal:"selections:selections:posts"`
	Note       coal.HasOne  `json:"-" bson:"-" coal:"note:notes:post"`
}

func (p *postModel) Validate() error {
	if p.Title == "error" {
		return E("error")
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
	Title     string        `json:"title" bson:"title"`
	Post      bson.ObjectId `json:"-" bson:"post_id" coal:"post:posts"`
}

type fooModel struct {
	coal.Base `json:"-" bson:",inline" coal:"foos"`
	Foo       bson.ObjectId   `json:"-" bson:"foo_id" coal:"foo:foos"`
	OptFoo    *bson.ObjectId  `json:"-" bson:"opt_foo_id" coal:"o-foo:foos"`
	Foos      []bson.ObjectId `json:"-" bson:"foo_ids" coal:"foos:foos"`
	Bar       bson.ObjectId   `json:"-" bson:"bar_id" coal:"bar:bars"`
	OptBar    *bson.ObjectId  `json:"-" bson:"opt_bar_id" coal:"o-bar:bars"`
	Bars      []bson.ObjectId `json:"-" bson:"bar_ids" coal:"bars:bars"`
}

type barModel struct {
	coal.Base `json:"-" bson:",inline" coal:"bars"`
	Foo       bson.ObjectId `json:"-" bson:"foo_id" coal:"foo:foos"`
}

var tester = NewTester(
	coal.MustCreateStore("mongodb://0.0.0.0:27017/test-fire"),
	&postModel{}, &commentModel{}, &selectionModel{}, &noteModel{}, &fooModel{}, &barModel{},
)

func TestMain(m *testing.M) {
	tr := transport.NewHTTPTransport("http://0.0.0.0:14268/api/traces?format=jaeger.thrift")
	defer tr.Close()

	tracer, closer := jaeger.NewTracer("test-fire",
		jaeger.NewConstSampler(true),
		jaeger.NewRemoteReporter(tr),
	)
	defer closer.Close()

	opentracing.SetGlobalTracer(tracer)

	m.Run()
}
