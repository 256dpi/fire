package fire

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

var session *mgo.Session

func init() {
	// set test mode
	gin.SetMode(gin.TestMode)

	// connect to local mongodb
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// store session globally
	session = sess
}

func getDB() *mgo.Database {
	// get db
	db := session.DB("")

	// clean database by removing all documents
	db.C("posts").RemoveAll(nil)
	db.C("comments").RemoveAll(nil)
	db.C("selections").RemoveAll(nil)

	return db
}

func buildServer() (*gin.Engine, *mgo.Database) {
	// get db
	db := getDB()

	// create router
	router := gin.New()

	// create app
	app := New(db, "")

	// add controllers
	app.Mount(&Controller{
		Model: &Post{},
	}, &Controller{
		Model: &Comment{},
	}, &Controller{
		Model: &Selection{},
	})

	// register routes
	app.Register(router)

	// return router
	return router, db
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
