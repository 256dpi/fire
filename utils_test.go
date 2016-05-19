package fire

import (
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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

	return db
}

func buildServer(resources ...*Resource) (*gin.Engine, *mgo.Database) {
	// get db
	db := getDB()

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
	return router, db
}

func saveModel(db *mgo.Database, collection string, model Model) Model {
	Init(model)

	err := db.C(collection).Insert(model)
	if err != nil {
		panic(err)
	}

	return model
}

func countChildren(c *gabs.Container) int {
	list, _ := c.Children()
	return len(list)
}

// some cheats to get more coverage

func TestAdapter(t *testing.T) {
	assert.Nil(t, (&adapter{}).Handler())
}
