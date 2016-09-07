package fire

import (
	"strings"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine"
	"github.com/labstack/echo/test"
	"gopkg.in/mgo.v2"
)

var session *mgo.Session

func init() {
	// connect to local mongodb
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// store session globally
	session = sess
}

func getDB() (*mgo.Session, *mgo.Database) {
	// get db
	db := session.DB("")

	// clean database by removing all documents
	db.C("posts").RemoveAll(nil)
	db.C("comments").RemoveAll(nil)
	db.C("selections").RemoveAll(nil)

	return session, db
}

func buildServer() (*echo.Echo, *mgo.Database) {
	// get db
	sess, db := getDB()

	// create router
	router := echo.New()

	// create set
	set := NewSet(sess, router, "")

	// add controllers
	set.Mount(&Controller{
		Model: &Post{},
	}, &Controller{
		Model: &Comment{},
	}, &Controller{
		Model: &Selection{},
	})

	// return router
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
