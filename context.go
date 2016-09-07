package fire

import (
	"github.com/gonfire/jsonapi"
	"github.com/labstack/echo"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// An Action describes the currently called action on the API.
type Action int

// All the available actions.
const (
	_ Action = iota
	List
	Find
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

	// The underlying JSON API request in progress.
	Request *jsonapi.Request

	// The underlying echo context.
	Echo echo.Context

	session *mgo.Session

	slice interface{}

	original Model
}

// Original will return the stored version of the model. This method is intended
// to be used to calculate the changed fields during an Update action.
//
// Note: The method will directly return any mgo errors and panic if being used
// during any other action than Update.
func (c *Context) Original() (Model, error) {
	if c.Action != Update {
		panic("Original can only be used during a Update action")
	}

	// return cached model
	if c.original != nil {
		return c.original, nil
	}

	// create a new model
	model := c.Model.Meta().Make()

	// read original document
	err := c.DB.C(c.Model.Meta().Collection).FindId(c.Model.ID()).One(model)
	if err != nil {
		return nil, err
	}

	// cache model
	c.original = Init(model)

	return c.original, nil
}

func (c *Context) clone() *Context {
	return &Context{
		DB:      c.DB,
		Request: c.Request,
		Echo:    c.Echo,
		session: c.session,
	}
}

func (c *Context) free() {
	c.session.Close()
}
