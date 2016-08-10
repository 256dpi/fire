package fire

import (
	"net/http"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/appleboy/gofight"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

const secret = "a-very-very-very-long-secret"

func TestBasicOperations(t *testing.T) {
	server, _ := buildServer(&Resource{
		Model: &Post{},
	})

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
	server, _ := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Comment{},
	})

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

func TestHasManyRelationshipFiltering(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Comment{},
	})

	// create posts
	post1 := saveModel(db, &Post{
		Title: "Post 1",
	})
	post2 := saveModel(db, &Post{
		Title: "Post 2",
	})

	// create comments
	saveModel(db, &Comment{
		Message: "Comment 1",
		PostID:  post1.ID(),
	})
	saveModel(db, &Comment{
		Message: "Comment 2",
		PostID:  post2.ID(),
	})

	r := gofight.New()

	// get related post
	r.GET("/posts/"+post1.ID().Hex()+"/comments").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, "comments", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Comment 1", obj.Path("attributes.message").Data().(string))
		})
}

func TestToOneRelationship(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Comment{},
	})

	// create post
	post := saveModel(db, &Post{
		Title: "Hello World!",
	})

	r := gofight.New()

	var link string

	// create relating post
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
							"id": "`+post.ID().Hex()+`"
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
			assert.Equal(t, post.ID().Hex(), obj.Path("relationships.post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.post.links.related").Data().(string))

			link = obj.Path("relationships.post.links.related").Data().(string)
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
			assert.Equal(t, "Hello World!", obj.Path("attributes.title").Data().(string))
		})
}

func TestFiltering(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	saveModel(db, &Post{
		Title: "post-1",
	})
	saveModel(db, &Post{
		Title: "post-2",
	})
	saveModel(db, &Post{
		Title: "post-3",
	})

	r := gofight.New()

	// get posts with single value filter
	r.GET("/posts?filter[title]=post-1").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "post-1", obj.Path("attributes.title").Data().(string))
		})

	// get posts with multi value filter
	r.GET("/posts?filter[title]=post-2,post-3").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 2, countChildren(json.Path("data")))
		})
}

func TestSorting(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	saveModel(db, &Post{
		Title: "2",
	})
	saveModel(db, &Post{
		Title: "1",
	})
	saveModel(db, &Post{
		Title: "3",
	})

	r := gofight.New()

	// get posts in ascending order
	r.GET("/posts?sort=title").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 3, countChildren(json.Path("data")))
			assert.Equal(t, "1", json.Path("data").Index(0).Path("attributes.title").Data().(string))
			assert.Equal(t, "2", json.Path("data").Index(1).Path("attributes.title").Data().(string))
			assert.Equal(t, "3", json.Path("data").Index(2).Path("attributes.title").Data().(string))
		})

	// get posts in descending order
	r.GET("/posts?sort=-title").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 3, countChildren(json.Path("data")))
			assert.Equal(t, "3", json.Path("data").Index(0).Path("attributes.title").Data().(string))
			assert.Equal(t, "2", json.Path("data").Index(1).Path("attributes.title").Data().(string))
			assert.Equal(t, "1", json.Path("data").Index(2).Path("attributes.title").Data().(string))
		})
}

func TestSparseFieldsets(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	saveModel(db, &Post{
		Title: "post-1",
	})

	r := gofight.New()

	// get posts with single value filter
	r.GET("/posts?fields[posts]=title").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, 1, countChildren(obj.Path("attributes")))
		})
}
