package fire

import (
	"net/http"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/appleboy/gofight"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

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

	var id string

	// create post
	r.POST("/posts").
		SetBody(`{
			"data": {
				"type": "posts",
				"attributes": {
			  		"title": "Post 1"
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Post 1", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "", obj.Path("attributes.text-body").Data().(string))

			id = obj.Path("id").Data().(string)
		})

	// get list of posts
	r.GET("/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.Equal(t, id, obj.Path("id").Data().(string))
			assert.Equal(t, "Post 1", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "", obj.Path("attributes.text-body").Data().(string))
		})

	// update post
	r.PATCH("/posts/"+id).
		SetBody(`{
			"data": {
				"type": "posts",
				"id": "`+id+`",
				"attributes": {
			  		"text-body": "Post 1 Text"
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.Equal(t, id, obj.Path("id").Data().(string))
			assert.Equal(t, "Post 1", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "Post 1 Text", obj.Path("attributes.text-body").Data().(string))
		})

	// get single post
	r.GET("/posts/"+id).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.Equal(t, id, obj.Path("id").Data().(string))
			assert.Equal(t, "Post 1", obj.Path("attributes.title").Data().(string))
			assert.Equal(t, "Post 1 Text", obj.Path("attributes.text-body").Data().(string))
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

func TestFiltering(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	saveModel(db, &Post{
		Title:     "post-1",
		Published: true,
	})
	saveModel(db, &Post{
		Title:     "post-2",
		Published: false,
	})
	saveModel(db, &Post{
		Title:     "post-3",
		Published: true,
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

	// get posts with boolean
	r.GET("/posts?filter[published]=true").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 2, countChildren(json.Path("data")))
		})

	// get posts with boolean
	r.GET("/posts?filter[published]=false").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
		})
}

func TestSorting(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	saveModel(db, &Post{
		Title: "post-2",
	})
	saveModel(db, &Post{
		Title: "post-1",
	})
	saveModel(db, &Post{
		Title: "post-3",
	})

	r := gofight.New()

	// get posts in ascending order
	r.GET("/posts?sort=title").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 3, countChildren(json.Path("data")))
			assert.Equal(t, "post-1", json.Path("data").Index(0).Path("attributes.title").Data().(string))
			assert.Equal(t, "post-2", json.Path("data").Index(1).Path("attributes.title").Data().(string))
			assert.Equal(t, "post-3", json.Path("data").Index(2).Path("attributes.title").Data().(string))
		})

	// get posts in descending order
	r.GET("/posts?sort=-title").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 3, countChildren(json.Path("data")))
			assert.Equal(t, "post-3", json.Path("data").Index(0).Path("attributes.title").Data().(string))
			assert.Equal(t, "post-2", json.Path("data").Index(1).Path("attributes.title").Data().(string))
			assert.Equal(t, "post-1", json.Path("data").Index(2).Path("attributes.title").Data().(string))
		})
}

func TestSparseFieldsets(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	saveModel(db, &Post{
		Title: "Post 1",
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

func TestHasManyRelationship(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Comment{},
	})

	// create existing post & comment
	post1 := saveModel(db, &Post{
		Title: "Post 1",
	})
	saveModel(db, &Comment{
		Message: "Comment 1",
		PostID:  post1.ID(),
	})

	// create new post
	post2 := saveModel(db, &Post{
		Title: "Post 2",
	})

	r := gofight.New()

	var link string

	// get single post
	r.GET("/posts/"+post2.ID().Hex()).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.NotEmpty(t, obj.Path("relationships.comments.links.related").Data().(string))

			link = obj.Path("relationships.comments.links.related").Data().(string)
		})

	assert.Equal(t, "/posts/"+post2.ID().Hex()+"/comments", link)

	// get empty list of related comments
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})

	var id string

	// create related comment
	r.POST("/comments").
		SetBody(`{
			"data": {
				"type": "comments",
				"attributes": {
			  		"message": "Comment 2"
				},
				"relationships": {
					"post": {
						"data": {
							"type": "posts",
							"id": "`+post2.ID().Hex()+`"
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
			assert.Equal(t, "Comment 2", obj.Path("attributes.message").Data().(string))
			assert.Equal(t, post2.ID().Hex(), obj.Path("relationships.post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.post.links.related").Data().(string))

			id = obj.Path("id").Data().(string)
		})

	// get list of related comments
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, "comments", obj.Path("type").Data().(string))
			assert.Equal(t, id, obj.Path("id").Data().(string))
			assert.Equal(t, "Comment 2", obj.Path("attributes.message").Data().(string))
			assert.Equal(t, post2.ID().Hex(), obj.Path("relationships.post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.post.links.related").Data().(string))
		})

	// get only relationship links
	r.GET("/posts/"+post2.ID().Hex()+"/relationships/comments").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json))               // expect only links
			assert.Equal(t, 2, countChildren(json.Path("links"))) // expect only related and self
		})

	// attempt to override relationship
	r.PATCH("/posts/"+post2.ID().Hex()+"/relationships/comments").
		SetBody(`{ "data": [] }`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			// TODO: This should be 403 Forbidden as the reference is not loaded.
			// See: https://github.com/manyminds/api2go/issues/260
			// Actually we shouldn't see a route for this at all.
			assert.Equal(t, http.StatusNoContent, r.Code)
		})
}

func TestToOneRelationship(t *testing.T) {
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

	r := gofight.New()

	var id, link string

	// create relating post
	r.POST("/comments").
		SetBody(`{
			"data": {
				"type": "comments",
				"attributes": {
			  		"message": "Comment 1"
				},
				"relationships": {
					"post": {
						"data": {
							"type": "posts",
							"id": "`+post1.ID().Hex()+`"
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
			assert.Equal(t, "Comment 1", obj.Path("attributes.message").Data().(string))
			assert.Equal(t, post1.ID().Hex(), obj.Path("relationships.post.data.id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.post.data.type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.post.links.related").Data().(string))

			id = obj.Path("id").Data().(string)
			link = obj.Path("relationships.post.links.related").Data().(string)
		})

	assert.Equal(t, "/comments/"+id+"/post", link)

	// get related post
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data").Index(0)

			// TODO: Why does the API return an array rather than an object?

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 1, countChildren(json.Path("data")))
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.Equal(t, post1.ID().Hex(), obj.Path("id").Data().(string))
			assert.Equal(t, "Post 1", obj.Path("attributes.title").Data().(string))
		})

	// get related post id only
	r.GET("/comments/"+id+"/relationships/post").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.Equal(t, post1.ID().Hex(), obj.Path("id").Data().(string))
		})

	// update relationship
	r.PATCH("/comments/"+id+"/relationships/post").
		SetBody(`{
			"data": {
				"type": "comments",
				"id": "`+post2.ID().Hex()+`"
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// fetch updated relationship
	r.GET("/comments/"+id+"/relationships/post").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj.Path("type").Data().(string))
			assert.Equal(t, post2.ID().Hex(), obj.Path("id").Data().(string))
		})
}

func TestToManyRelationship(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Selection{},
	})

	// create posts
	post1 := saveModel(db, &Post{
		Title: "Post 1",
	})
	post2 := saveModel(db, &Post{
		Title: "Post 2",
	})
	post3 := saveModel(db, &Post{
		Title: "Post 3",
	})

	r := gofight.New()

	var id, link string

	// create selection
	r.POST("/selections").
		SetBody(`{
			"data": {
				"type": "selections",
				"attributes": {
			  		"name": "Selection 1"
				},
				"relationships": {
					"posts": {
						"data": [
							{
								"type": "posts",
								"id": "`+post1.ID().Hex()+`"
							},
							{
								"type": "posts",
								"id": "`+post2.ID().Hex()+`"
							}
						]
					}
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj := json.Path("data")

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.Equal(t, "selections", obj.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj.Path("id").Data().(string)))
			assert.Equal(t, "Selection 1", obj.Path("attributes.name").Data().(string))
			assert.Equal(t, post1.ID().Hex(), obj.Path("relationships.posts.data").Index(0).Path("id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.posts.data").Index(0).Path("type").Data().(string))
			assert.Equal(t, post2.ID().Hex(), obj.Path("relationships.posts.data").Index(1).Path("id").Data().(string))
			assert.Equal(t, "posts", obj.Path("relationships.posts.data").Index(1).Path("type").Data().(string))
			assert.NotEmpty(t, obj.Path("relationships.posts.links.related").Data().(string))

			id = obj.Path("id").Data().(string)
			link = obj.Path("relationships.posts.links.related").Data().(string)
		})

	assert.Equal(t, "/selections/"+id+"/posts", link)

	// get related post
	r.GET(link).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj1 := json.Path("data").Index(0)
			obj2 := json.Path("data").Index(1)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 2, countChildren(json.Path("data")))
			assert.Equal(t, "posts", obj1.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj1.Path("id").Data().(string)))
			assert.Equal(t, "Post 1", obj1.Path("attributes.title").Data().(string))
			assert.Equal(t, "posts", obj2.Path("type").Data().(string))
			assert.True(t, bson.IsObjectIdHex(obj2.Path("id").Data().(string)))
			assert.Equal(t, "Post 2", obj2.Path("attributes.title").Data().(string))
		})

	// get related post ids only
	r.GET("/selections/"+id+"/relationships/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj1 := json.Path("data").Index(0)
			obj2 := json.Path("data").Index(1)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj1.Path("type").Data().(string))
			assert.Equal(t, post1.ID().Hex(), obj1.Path("id").Data().(string))
			assert.Equal(t, "posts", obj2.Path("type").Data().(string))
			assert.Equal(t, post2.ID().Hex(), obj2.Path("id").Data().(string))
		})

	// update relationship
	r.PATCH("/selections/"+id+"/relationships/posts").
		SetBody(`{
			"data": [
				{
					"type": "comments",
					"id": "`+post3.ID().Hex()+`"
				}
			]
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// get updated related post ids only
	r.GET("/selections/"+id+"/relationships/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj1 := json.Path("data").Index(0)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj1.Path("type").Data().(string))
			assert.Equal(t, post3.ID().Hex(), obj1.Path("id").Data().(string))
		})

	// add relationship
	r.POST("/selections/"+id+"/relationships/posts").
		SetBody(`{
			"data": [
				{
					"type": "comments",
					"id": "`+post1.ID().Hex()+`"
				}
			]
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusNoContent, r.Code)
		assert.Equal(t, "", r.Body.String())
	})

	// get related post ids only
	r.GET("/selections/"+id+"/relationships/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			obj1 := json.Path("data").Index(0)
			obj2 := json.Path("data").Index(1)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "posts", obj1.Path("type").Data().(string))
			assert.Equal(t, post3.ID().Hex(), obj1.Path("id").Data().(string))
			assert.Equal(t, "posts", obj2.Path("type").Data().(string))
			assert.Equal(t, post1.ID().Hex(), obj2.Path("id").Data().(string))
		})

	// remove relationship
	r.DELETE("/selections/"+id+"/relationships/posts").
		SetBody(`{
			"data": [
				{
					"type": "comments",
					"id": "`+post3.ID().Hex()+`"
				},
				{
					"type": "comments",
					"id": "`+post1.ID().Hex()+`"
				}
			]
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		assert.Equal(t, http.StatusNoContent, r.Code)
		assert.Equal(t, "", r.Body.String())
	})

	// get empty related post ids list
	r.GET("/selections/"+id+"/relationships/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
		json, _ := gabs.ParseJSONBuffer(r.Body)

		assert.Equal(t, http.StatusOK, r.Code)
		assert.Equal(t, 0, countChildren(json.Path("data")))
	})
}

func TestEmptyToManyRelationship(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Selection{},
	})

	// create posts
	post := saveModel(db, &Post{
		Title: "Post 1",
	})

	// create selection
	selection := saveModel(db, &Selection{
		Name: "Selection 1",
	})

	r := gofight.New()

	// get related posts
	r.GET("/selections/"+selection.ID().Hex()+"/posts").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 0, countChildren(json.Path("data")))
		})

	// get related selections
	r.GET("/posts/"+post.ID().Hex()+"/selections").
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)

			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, 0, countChildren(json.Path("data")))
		})
}
