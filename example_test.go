package fire

import (
	"fmt"

	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	Base       `bson:",inline" fire:"posts"`
	Title      string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published  bool    `json:"published" valid:"-" fire:"filterable"`
	TextBody   string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments   HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	Selections HasMany `json:"-" valid:"-" bson:"-" fire:"selections:selections:posts"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	Base    `bson:",inline" fire:"selections:selections"`
	Name    string          `json:"name" valid:"required"`
	PostIDs []bson.ObjectId `json:"-" valid:"-" bson:"post_ids" fire:"posts:posts"`
}

func Example() {
	// connect to database
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// defer close
	defer sess.Close()

	// get db
	db := sess.DB("")

	// create app
	app := New(db, "api")

	// add controllers
	app.Mount(&Controller{
		Model: &Post{},
	}, &Controller{
		Model: &Comment{},
	}, &Controller{
		Model: &Selection{},
	})

	// create router
	router := echo.New()

	// register api
	app.Register(router)

	fmt.Println("ready!")

	// run server
	// err = router.Run("localhost:8080")
	// if err != nil {
	//	 panic(err)
	// }

	// Output:
	// ready!
}
