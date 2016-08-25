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
	Base    `bson:",inline" fire:"selection:selections"`
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

	// create router
	router := gin.New()

	// register api
	endpoint.Register("api", router)

	fmt.Println("server ready to run")

	// run server
	//err = router.Run("localhost:8080")
	//if err != nil {
	//	panic(err)
	//}

	// Output:
	// server ready to run
}
