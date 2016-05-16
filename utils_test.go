package fire

import (
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func buildServer(resources ...*Resource) (*gin.Engine, *mgo.Database, func()) {
	// connect to local mongodb
	session, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// get db
	db := session.DB("")

	// clean database by dropping it
	db.DropDatabase()

	// create new router and endpoint
	router := gin.Default()
	endpoint := NewEndpoint(db)

	// add all supplied resources
	for _, res := range resources {
		endpoint.AddResource(res)
	}

	// register routes
	endpoint.Register("", router)

	// return router
	return router, db, func() {
		session.Close()

	}
}

func saveModel(db *mgo.Database, collection string, model Model) Model {
	Init(model)

	err := db.C(collection).Insert(model)
	if err != nil {
		panic(err)
	}

	return model
}
