package main

import (
	"github.com/gonfire/fire"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"gopkg.in/mgo.v2"
)

type Post struct {
	fire.Base `bson:",inline" fire:"posts"`
	Title     string `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published bool   `json:"published" valid:"-" fire:"filterable"`
	TextBody  string `json:"text-body" valid:"-" bson:"text_body"`
}

func main() {
	// connect to database
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// defer close
	defer sess.Close()

	// get db
	db := sess.DB("")

	// create router
	router := echo.New()

	// create a new controller set
	set := fire.NewSet(db, router, "")

	// create and mount controller
	set.Mount(&fire.Controller{
		Model: &Post{},
	})

	// run server
	router.Run(standard.New("0.0.0.0:4000"))
}
