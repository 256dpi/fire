package fire

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	Base       `bson:",inline" fire:"post:posts"`
	Title      string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	Published  bool    `json:"published" valid:"-" fire:"filterable"`
	TextBody   string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments   HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
	Selections HasMany `json:"-" valid:"-" bson:"-" fire:"selections:selections"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

type Selection struct {
	Base    `bson:",inline" fire:"selection:selections:selections"`
	Name    string          `json:"name" valid:"required"`
	PostIDs []bson.ObjectId `json:"-" valid:"-" bson:"post_ids" fire:"posts:posts"`
}

func Example() {
	// connect to database
	sess, err := mgo.Dial("mongodb://0.0.0.0:27017/fire")
	if err != nil {
		panic(err)
	}

	// defer close
	defer sess.Close()

	// get db
	db := sess.DB("")

	// create endpoint
	endpoint := NewEndpoint(db)

	// add post
	endpoint.AddResource(&Resource{
		Model: &Post{},
	})

	// add comment
	endpoint.AddResource(&Resource{
		Model: &Comment{},
	})

	// add selection
	endpoint.AddResource(&Resource{
		Model: &Selection{},
	})

	// create router
	router := gin.New()

	// register api
	endpoint.Register("api", router)

	// print routes
	for _, route := range router.Routes() {
		fmt.Printf("%s: %s\n", route.Method, route.Path)
	}

	// run server
	// err = router.Run("localhost:8080")
	// if err != nil {
	//	 panic(err)
	// }

	// Output:
	// OPTIONS: /api/posts
	// OPTIONS: /api/posts/:id
	// OPTIONS: /api/comments
	// OPTIONS: /api/comments/:id
	// OPTIONS: /api/selections
	// OPTIONS: /api/selections/:id
	// GET: /api/posts
	// GET: /api/posts/:id
	// GET: /api/posts/:id/relationships/comments
	// GET: /api/posts/:id/relationships/selections
	// GET: /api/posts/:id/comments
	// GET: /api/posts/:id/selections
	// GET: /api/comments
	// GET: /api/comments/:id
	// GET: /api/comments/:id/relationships/parent
	// GET: /api/comments/:id/relationships/post
	// GET: /api/comments/:id/parent
	// GET: /api/comments/:id/post
	// GET: /api/selections
	// GET: /api/selections/:id
	// GET: /api/selections/:id/relationships/posts
	// GET: /api/selections/:id/posts
	// PATCH: /api/posts/:id
	// PATCH: /api/posts/:id/relationships/comments
	// PATCH: /api/posts/:id/relationships/selections
	// PATCH: /api/comments/:id
	// PATCH: /api/comments/:id/relationships/parent
	// PATCH: /api/comments/:id/relationships/post
	// PATCH: /api/selections/:id
	// PATCH: /api/selections/:id/relationships/posts
	// POST: /api/posts
	// POST: /api/posts/:id/relationships/comments
	// POST: /api/posts/:id/relationships/selections
	// POST: /api/selections
	// POST: /api/selections/:id/relationships/posts
	// POST: /api/comments
	// DELETE: /api/posts/:id
	// DELETE: /api/posts/:id/relationships/comments
	// DELETE: /api/posts/:id/relationships/selections
	// DELETE: /api/selections/:id
	// DELETE: /api/selections/:id/relationships/posts
	// DELETE: /api/comments/:id

	// TODO: EditToManyRelations routes should not be generated for has many
	// relationships.
}
