package fire

import (
	"net/http"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/appleboy/gofight"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type Post struct {
	Base     `bson:",inline" fire:"post:posts"`
	Title    string         `json:"title" valid:"required"`
	TextBody string         `json:"text-body" valid:"-" bson:"text_body"`
	NextPost *bson.ObjectId `json:"-" valid:"-" bson:"next_post_id" fire:"next-post:posts"`
	Comments HasMany        `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string        `json:"message" valid:"required"`
	PostID  bson.ObjectId `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

func TestBasicOperations(t *testing.T) {
	server, _, close := buildServer(&Resource{
		Model:      &Post{},
		Collection: "posts",
	})

	defer close()

	r := gofight.New()

	// get empty list of posts
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})

	// create post
	r.POST("/posts").
		SetBody(`{
			"data": {
				"type": "posts",
				"attributes": {
			  		"title": "Hello World!"
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Hello World!", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "", obj.Path("attributes.text-body").Data().(string))
		})

	var id string

	// get list of posts
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Hello World!", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "", obj.Path("attributes.text-body").Data().(string))

			id = obj.Path("id").Data().(string)
		})

	// update post
	r.PATCH("/posts/"+id).
		SetBody(`{
			"data": {
				"type": "posts",
				"id": "`+id+`",
				"attributes": {
			  		"text-body": "Some Text..."
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Hello World!", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "Some Text...", obj.Path("attributes.text-body").Data().(string))
		})

	// get single post
	r.GET("/posts/"+id).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Hello World!", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "Some Text...", obj.Path("attributes.text-body").Data().(string))
		})

	// delete post
	r.DELETE("/posts/"+id).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// get empty list of posts
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}

func TestHasManyRelationship(t *testing.T) {
	server, _, close := buildServer(&Resource{
		Model:      &Post{},
		Collection: "posts",
	}, &Resource{
		Model:      &Comment{},
		Collection: "comments",
	})

	defer close()

	r := gofight.New()

	var id string
	var link string

	// create post
	r.POST("/posts").
		SetBody(`{
			"data": {
				"type": "posts",
				"attributes": {
			  		"title": "Hello World!"
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.NotEmpty(t, obj.Path("relationships.comments.links.related").Data().(string))

			id = obj.Path("id").Data().(string)
			link = obj.Path("relationships.comments.links.related").Data().(string)
		})

	// get empty list of related comments
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})

	// create related comment
	r.POST("/comments").
		SetBody(`{
			"data": {
				"type": "comments",
				"attributes": {
			  		"message": "Amazing Thing!"
				},
				"relationships": {
					"post": {
						"data": {
							"type": "posts",
							"id": "`+id+`"
						}
					}
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.Equal(t, "comments", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Amazing Thing!", obj.Path("attributes.message").Data().(string))
			assert.Equal(t, id, obj.Path("relationships.post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.post.links.related").Data().(string))
		})

	// get list of related comments
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, "comments", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Amazing Thing!", obj.Path("attributes.message").Data().(string))
			assert.Equal(t, id, obj.Path("relationships.post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.post.links.related").Data().(string))
		})
}

func TestToOneRelationship(t *testing.T) {
	server, db, close := buildServer(&Resource{
		Model:      &Post{},
		Collection: "posts",
	})

	defer close()

	r := gofight.New()

	// create post
	post := saveModel(db, "posts", &Post{
		Title: "Hello World!",
	})

	var link string

	// create relating post
	r.POST("/posts").
		SetBody(`{
			"data": {
				"type": "posts",
				"attributes": {
			  		"title": "Amazing Thing!"
				},
				"relationships": {
					"next-post": {
						"data": {
							"type": "posts",
							"id": "`+post.GetID()+`"
						}
					}
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Amazing Thing!", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, post.GetID(), obj.Path("relationships.next-post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.next-post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.next-post.links.related").Data().(string))

			link = obj.Path("relationships.next-post.links.related").Data().(string)
		})

	// get related post
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Amazing Thing!", obj.Path("attributes.title").Data().(string))
		})
}
