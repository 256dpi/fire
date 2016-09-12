package main

import (
	"github.com/gonfire/fire"
	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
)

type post struct {
	fire.Base `json:"-" bson:",inline" fire:"posts"`
	Title     string `json:"title" valid:"required" fire:"filterable,sortable"`
}

func main() {
	// create pool
	pool := fire.NewClonePool("mongodb://0.0.0.0:27017/fire-test-echo")

	// create router
	router := echo.New()

	// create a new controller group
	group := fire.NewControllerGroup("api")

	// add controller
	group.Add(&fire.Controller{
		Model: &post{},
		Pool:  pool,
	})

	// register group
	group.Register(router)

	// run server
	router.Run(standard.New("0.0.0.0:4000"))
}
