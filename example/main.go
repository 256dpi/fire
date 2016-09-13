// This example demonstrates the usage of the fire framework to build a simple
// JSON API.
package main

import (
	"github.com/gonfire/fire"
	"github.com/gonfire/fire/components"
	"github.com/gonfire/fire/jsonapi"
	"github.com/gonfire/fire/model"
)

type post struct {
	model.Base `json:"-" bson:",inline" fire:"posts"`
	Title      string `json:"title" valid:"required" fire:"filterable,sortable"`
}

func main() {
	// create pool
	pool := fire.NewPool("mongodb://0.0.0.0:27017/fire-test-echo")

	// create a new app
	app := fire.New()

	// create a new group
	group := jsonapi.New("api")

	// add controller
	group.Add(&jsonapi.Controller{
		Model: &post{},
		Pool:  pool,
	})

	// mount protector
	app.Mount(components.DefaultProtector())

	// mount group
	app.Mount(group)

	// mount inspector
	app.Mount(fire.DefaultInspector(app))

	// run server
	app.Start("0.0.0.0:4000")
}
