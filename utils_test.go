package fire

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/engine/standard"
	"github.com/labstack/echo/test"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	Base       `json:"-" bson:",inline" fire:"posts"`
	Title      string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published  bool    `json:"published" valid:"-" fire:"filterable"`
	TextBody   string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments   HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	Selections HasMany `json:"-" valid:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	Base    `json:"-" bson:",inline" fire:"comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	Base    `json:"-" bson:",inline" fire:"selections:selections"`
	Name    string          `json:"name" valid:"required"`
	PostIDs []bson.ObjectId `json:"-" valid:"-" bson:"post_ids" fire:"posts:posts"`
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

func (c *testComponent) Inspect() string {
	return "This is a test component\n"
}

var session *mgo.Session

func init() {
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	session = sess
}

func getCleanDB() *mgo.Database {
	db := session.DB("")

	db.C("posts").RemoveAll(nil)
	db.C("comments").RemoveAll(nil)
	db.C("selections").RemoveAll(nil)

	return db
}

func buildServer() (*echo.Echo, *mgo.Database) {
	db := getCleanDB()
	router := echo.New()
	group := NewControllerGroup("")

	group.Add(&Controller{
		Model: &Post{},
		Pool:  &clonePool{root: session},
	}, &Controller{
		Model: &Comment{},
		Pool:  &clonePool{root: session},
	}, &Controller{
		Model: &Selection{},
		Pool:  &clonePool{root: session},
	})

	group.Register(router)

	return router, db
}

func testRequest(e *echo.Echo, method, path string, headers map[string]string, payload string, callback func(*test.ResponseRecorder, engine.Request)) {
	req := test.NewRequest(method, path, strings.NewReader(payload))
	rec := test.NewResponseRecorder()

	for k, v := range headers {
		req.Header().Set(k, v)
	}

	e.ServeHTTP(req, rec)

	callback(rec, req)
}

func saveModel(db *mgo.Database, model Model) Model {
	Init(model)

	err := db.C(model.Meta().Collection).Insert(model)
	if err != nil {
		panic(err)
	}

	return model
}

func findLastModel(db *mgo.Database, model Model) Model {
	Init(model)

	err := db.C(model.Meta().Collection).Find(nil).Sort("-_id").One(model)
	if err != nil {
		panic(err)
	}

	return model
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
