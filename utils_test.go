package fire

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gonfire/fire/model"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Title      string        `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published  bool          `json:"published" valid:"-" fire:"filterable"`
	TextBody   string        `json:"text-body" valid:"-" bson:"text_body"`
	Comments   model.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	Selections model.HasMany `json:"-" valid:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	model.Base `json:"-" bson:",inline" fire:"comments"`
	Message    string         `json:"message" valid:"required"`
	Parent     *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	model.Base `json:"-" bson:",inline" fire:"selections:selections"`
	Name       string          `json:"name" valid:"required"`
	PostIDs    []bson.ObjectId `json:"-" valid:"-" bson:"post_ids" fire:"posts:posts"`
}

type testComponent struct{}

func (c *testComponent) Register(router *echo.Echo) {
	router.GET("/", func(ctx echo.Context) error {
		ctx.String(200, "OK")
		return nil
	})

	router.GET("/foo", func(ctx echo.Context) error {
		ctx.String(200, "OK")
		return nil
	})

	router.GET("/error", func(ctx echo.Context) error {
		return errors.New("error")
	})
}

func (c *testComponent) Setup(router *echo.Echo) error {
	return nil
}

func (c *testComponent) Teardown() error {
	return nil
}

func (c *testComponent) Inspect() ComponentInfo {
	return ComponentInfo{
		Name: "testComponent",
		Settings: Map{
			"foo": "bar",
		},
	}
}

func runApp(app *Application) (chan struct{}, string) {
	listener, err := net.Listen("tcp", ":")
	if err != nil {
		panic(err)
	}

	server := standard.WithConfig(engine.Config{
		Listener: listener,
	})

	go func() {
		err := app.Run(server)
		if err != nil {
			panic(err)
		}
	}()

	done := make(chan struct{})

	go func(done chan struct{}) {
		<-done
		listener.Close()
	}(done)

	time.Sleep(50 * time.Millisecond)

	return done, fmt.Sprintf("http://%s", listener.Addr().String())
}
