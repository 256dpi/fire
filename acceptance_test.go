package fire

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/appleboy/gofight"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

type Application struct {
	Base     `bson:",inline" fire:"application:applications"`
	Name     string `json:"name" valid:"required"`
	Key      string `json:"key" valid:"required" fire:"identifiable"`
	Secret   []byte `json:"secret" valid:"required" fire:"verifiable"`
	Callback string `json:"callback" valid:"required" fire:"callable"`
}

type User struct {
	Base     `bson:",inline" fire:"user:users"`
	FullName string `json:"full_name" valid:"required"`
	Email    string `json:"email" valid:"required" fire:"identifiable"`
	Password []byte `json:"-" valid:"required" fire:"verifiable"`
}

type Post struct {
	Base     `bson:",inline" fire:"post:posts"`
	Title    string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	Base    `bson:",inline" fire:"comment:comments"`
	Message string         `json:"message" valid:"required"`
	Parent  *bson.ObjectId `json:"parent" valid:"-" fire:"parent:comments"`
	PostID  bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}

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

func TestPasswordGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), &User{}, &Application{}, "a-very-very-very-long-secret")
	authenticator.EnablePasswordGrant()

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer(),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:   "Test Application",
		Key:    "key1",
		Secret: authenticator.MustHashPassword("secret"),
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user1@example.com",
		Password: authenticator.MustHashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key1", "secret")).
		SetFORM(gofight.H{
			"grant_type": "password",
			"username":   "user1@example.com",
			"password":   "secret",
			"scope":      "fire",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "fire", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}

func TestCredentialsGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), &User{}, &Application{}, "a-very-very-very-long-secret")
	authenticator.EnableCredentialsGrant()

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer(),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:   "Test Application",
		Key:    "key2",
		Secret: authenticator.MustHashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/token").
		SetHeader(basicAuth("key2", "secret")).
		SetFORM(gofight.H{
			"grant_type": "client_credentials",
			"scope":      "fire",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			json, _ := gabs.ParseJSONBuffer(r.Body)
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "3600", json.Path("expires_in").Data().(string))
			assert.Equal(t, "fire", json.Path("scope").Data().(string))
			assert.Equal(t, "bearer", json.Path("token_type").Data().(string))

			token = json.Path("access_token").Data().(string)
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}

func TestImplicitGrant(t *testing.T) {
	authenticator := NewAuthenticator(getDB(), &User{}, &Application{}, "a-very-very-very-long-secret")
	authenticator.EnableImplicitGrant()

	server, db := buildServer(&Resource{
		Model:      &Post{},
		Authorizer: authenticator.Authorizer(),
	})

	authenticator.Register("auth", server)

	// create application
	saveModel(db, &Application{
		Name:     "Test Application",
		Key:      "key3",
		Secret:   authenticator.MustHashPassword("secret"),
		Callback: "https://0.0.0.0:8080/auth/callback",
	})

	// create user
	saveModel(db, &User{
		FullName: "Test User",
		Email:    "user2@example.com",
		Password: authenticator.MustHashPassword("secret"),
	})

	r := gofight.New()

	var token string

	// get access token
	r.POST("/auth/authorize").
		SetFORM(gofight.H{
			"response_type": "token",
			"redirect_uri":  "https://0.0.0.0:8080/auth/callback",
			"client_id":     "key3",
			"state":         "state1234",
			"scope":         "fire",
			"username":      "user2@example.com",
			"password":      "secret",
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			loc, err := url.Parse(r.HeaderMap.Get("Location"))
			assert.NoError(t, err)

			vals, err := url.ParseQuery(loc.Fragment)
			assert.NoError(t, err)

			assert.Equal(t, http.StatusFound, r.Code)
			assert.Equal(t, "3600", vals.Get("expires_in"))
			assert.Equal(t, "fire", vals.Get("scope"))
			assert.Equal(t, "bearer", vals.Get("token_type"))

			token = vals.Get("access_token")
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(bearerAuth(token)).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, `{"data":[]}`, r.Body.String())
		})
}
