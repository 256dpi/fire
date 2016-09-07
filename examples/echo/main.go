package main

import (
	"github.com/gonfire/fire"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"gopkg.in/mgo.v2"
)

type post struct {
	fire.Base `bson:",inline" fire:"posts"`
	Title     string `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
}

func main() {
	// connect to database
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire-test-echo")
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
	set := fire.NewSet(db, router, "api")

	// create and mount controller
	set.Mount(&fire.Controller{
		Model: &post{},
	})

	// run server
	router.Run(standard.New("0.0.0.0:4000"))
}
