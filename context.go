package fire

import (
	"github.com/gin-gonic/gin"
	"github.com/manyminds/api2go"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// An Action describes the currently called action on the API.
type Action int

// All the available actions.
const (
	FindAll Action = iota
	FindOne
	Create
	Update
	Delete
)

// A Context provides useful contextual information.
type Context struct {
	// The current action in process.
	Action Action

	// The Model that will be saved during Create or Update.
	Model Model

	// The query that will be used during FindAll, FindOne, Update or Delete.
	// On FindOne, Update and Delete, the "_id" key is preset to the document ID.
	// On FindAll all field filters and relationship filters are preset.
	Query bson.M

	// The sorting that will be used during FindAll.
	Sorting []string

	// The db used to query.
	DB *mgo.Database

	// The underlying gin.context.
	GinContext *gin.Context

	// The underlying api2go.Request.
	API2GoReq *api2go.Request

	db       *mgo.Database
	original Model
}
