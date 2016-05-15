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

func TestBlog(t *testing.T) {
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
		
			assert.Equal(t, http.StatusCreated, r.Code)
			assert.Equal(t, "posts", json.Path("data.type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(json.Path("data.id").Data().(string)))
			assert.Equal(t, "Hello World!", json.Path("data.attributes.title").Data().(string))
		})
}
