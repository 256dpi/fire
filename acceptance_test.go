package fire

import (
	"testing"
	"net/http"

	"gopkg.in/mgo.v2/bson"
	"github.com/stretchr/testify/assert"
	"github.com/appleboy/gofight"
	"github.com/Jeffail/gabs"
)

type Post struct {
	Base     `bson:",inline" fire:"post:posts"`
	Title    string  `json:"title" valid:"required"`
	TextBody string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string        `json:"message" valid:"required"`
	PostID  bson.ObjectId `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

var PostResource = &Resource{
	Model: &Post{},
	Collection: "posts",
}

var CommentResource = &Resource{
	Model: &Comment{},
	Collection: "comments",
}

func TestPosts(t *testing.T) {
	server := buildServer(PostResource, CommentResource)

	r := gofight.New()

	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})

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

	r.PATCH("/posts/" + id).
		SetBody(`{
			"data": {
				"type": "posts",
				"id": "` + id + `",
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

	r.GET("/posts/" + id).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Hello World!", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "Some Text...", obj.Path("attributes.text-body").Data().(string))
		})

	r.DELETE("/posts/" + id).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, `{"data":[]}`, r.Body.String())
	})
}
