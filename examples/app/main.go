package main

import "github.com/gonfire/fire"

type post struct {
	fire.Base `bson:",inline" fire:"posts"`
	Title     string `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
}

func main() {
	// create a new app
	app := fire.New("mongodb://0.0.0.0:27017/fire-test-app", "api")

	// create and mount controller
	app.Mount(&fire.Controller{
		Model: &post{},
	})

	// enable dev mode
	app.EnableDevMode()

	// run server
	app.Start("0.0.0.0:4000")
}
