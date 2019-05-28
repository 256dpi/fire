package fire

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/256dpi/fire/coal"
)

type postModel struct {
	coal.Base  `json:"-" bson:",inline" coal:"posts"`
	Title      string       `json:"title" bson:"title"`
	Published  bool         `json:"published"`
	TextBody   string       `json:"text-body" bson:"text_body"`
	Deleted    *time.Time   `json:"-" bson:"deleted_at" coal:"fire-soft-delete"`
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
	Message   string              `json:"message"`
	Deleted   *time.Time          `json:"-" bson:"deleted_at" coal:"fire-soft-delete"`
	Parent    *primitive.ObjectID `json:"-" bson:"parent_id" coal:"parent:comments"`
	Post      primitive.ObjectID  `json:"-" bson:"post_id" coal:"post:posts"`
}

type selectionModel struct {
	coal.Base   `json:"-" bson:",inline" coal:"selections:selections"`
	Name        string               `json:"name"`
	CreateToken string               `json:"create-token,omitempty" bson:"create_token" coal:"fire-idempotent-create"`
	UpdateToken string               `json:"update-token,omitempty" bson:"update_token" coal:"fire-consistent-update"`
	Posts       []primitive.ObjectID `json:"-" bson:"post_ids" coal:"posts:posts"`
}

type noteModel struct {
	coal.Base `json:"-" bson:",inline" coal:"notes"`
	Title     string             `json:"title" bson:"title"`
	Post      primitive.ObjectID `json:"-" bson:"post_id" coal:"post:posts"`
}

type fooModel struct {
	coal.Base `json:"-" bson:",inline" coal:"foos"`
	Foo       primitive.ObjectID   `json:"-" bson:"foo_id" coal:"foo:foos"`
	OptFoo    *primitive.ObjectID  `json:"-" bson:"opt_foo_id" coal:"o-foo:foos"`
	Foos      []primitive.ObjectID `json:"-" bson:"foo_ids" coal:"foos:foos"`
	Bar       primitive.ObjectID   `json:"-" bson:"bar_id" coal:"bar:bars"`
	OptBar    *primitive.ObjectID  `json:"-" bson:"opt_bar_id" coal:"o-bar:bars"`
	Bars      []primitive.ObjectID `json:"-" bson:"bar_ids" coal:"bars:bars"`
}

type barModel struct {
	coal.Base `json:"-" bson:",inline" coal:"bars"`
	Foo       primitive.ObjectID `json:"-" bson:"foo_id" coal:"foo:foos"`
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
