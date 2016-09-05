package fire

import (
	"net/http"
	"testing"

	"github.com/appleboy/gofight"
	"github.com/gonfire/jsonapi"
	"github.com/stretchr/testify/assert"
	"gopkg.in/mgo.v2/bson"
)

func TestBasicOperations(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	r := gofight.New()

	// get empty list of posts
	r.GET("/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})

	var id string

	// create post
	r.POST("/posts").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
		SetBody(`{
			"data": {
				"type": "posts",
				"attributes": {
			  		"title": "Post 1"
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			post := findLastModel(db, &Post{})
			id = post.ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+id+`",
					"attributes": {
						"title": "Post 1",
						"published": false,
						"text-body": ""
					},
					"relationships": {
						"comments": {
							"links": {
								"self": "/posts/`+id+`/relationships/comments",
								"related": "/posts/`+id+`/comments"
							}
						},
						"selections": {
							"links": {
								"self": "/posts/`+id+`/relationships/selections",
								"related": "/posts/`+id+`/selections"
							}
						}
					}
				},
				"links": {
					"self": "/posts/`+id+`"
				}
			}`, r.Body.String())
		})

	// get list of posts
	r.GET("/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+id+`",
						"attributes": {
							"title": "Post 1",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+id+`/relationships/comments",
									"related": "/posts/`+id+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+id+`/relationships/selections",
									"related": "/posts/`+id+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})

	// update post
	r.PATCH("/posts/"+id).
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
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
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+id+`",
					"attributes": {
						"title": "Post 1",
						"published": false,
						"text-body": "Post 1 Text"
					},
					"relationships": {
						"comments": {
							"links": {
								"self": "/posts/`+id+`/relationships/comments",
								"related": "/posts/`+id+`/comments"
							}
						},
						"selections": {
							"links": {
								"self": "/posts/`+id+`/relationships/selections",
								"related": "/posts/`+id+`/selections"
							}
						}
					}
				},
				"links": {
					"self": "/posts/`+id+`"
				}
			}`, r.Body.String())
		})

	// get single post
	r.GET("/posts/"+id).
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+id+`",
					"attributes": {
						"title": "Post 1",
						"published": false,
						"text-body": "Post 1 Text"
					},
					"relationships": {
						"comments": {
							"links": {
								"self": "/posts/`+id+`/relationships/comments",
								"related": "/posts/`+id+`/comments"
							}
						},
						"selections": {
							"links": {
								"self": "/posts/`+id+`/relationships/selections",
								"related": "/posts/`+id+`/selections"
							}
						}
					}
				},
				"links": {
					"self": "/posts/`+id+`"
				}
			}`, r.Body.String())
		})

	// delete post
	r.DELETE("/posts/"+id).
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// get empty list of posts
	r.GET("/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})
}

func TestFiltering(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts
	post1 := saveModel(db, &Post{
		Title:     "post-1",
		Published: true,
	}).ID().Hex()
	post2 := saveModel(db, &Post{
		Title:     "post-2",
		Published: false,
	}).ID().Hex()
	post3 := saveModel(db, &Post{
		Title:     "post-3",
		Published: true,
	}).ID().Hex()

	r := gofight.New()

	// get posts with single value filter
	r.GET("/posts?filter[title]=post-1").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post1+`",
						"attributes": {
							"title": "post-1",
							"published": true,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post1+`/relationships/comments",
									"related": "/posts/`+post1+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post1+`/relationships/selections",
									"related": "/posts/`+post1+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})

	// get posts with multi value filter
	r.GET("/posts?filter[title]=post-2,post-3").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post2+`",
						"attributes": {
							"title": "post-2",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post2+`/relationships/comments",
									"related": "/posts/`+post2+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post2+`/relationships/selections",
									"related": "/posts/`+post2+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post3+`",
						"attributes": {
							"title": "post-3",
							"published": true,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post3+`/relationships/comments",
									"related": "/posts/`+post3+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post3+`/relationships/selections",
									"related": "/posts/`+post3+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})

	// get posts with boolean
	r.GET("/posts?filter[published]=true").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post1+`",
						"attributes": {
							"title": "post-1",
							"published": true,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post1+`/relationships/comments",
									"related": "/posts/`+post1+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post1+`/relationships/selections",
									"related": "/posts/`+post1+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post3+`",
						"attributes": {
							"title": "post-3",
							"published": true,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post3+`/relationships/comments",
									"related": "/posts/`+post3+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post3+`/relationships/selections",
									"related": "/posts/`+post3+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})

	// get posts with boolean
	r.GET("/posts?filter[published]=false").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post2+`",
						"attributes": {
							"title": "post-2",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post2+`/relationships/comments",
									"related": "/posts/`+post2+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post2+`/relationships/selections",
									"related": "/posts/`+post2+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})
}

func TestSorting(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	})

	// create posts in random order
	post2 := saveModel(db, &Post{
		Title: "post-2",
	}).ID().Hex()
	post1 := saveModel(db, &Post{
		Title: "post-1",
	}).ID().Hex()
	post3 := saveModel(db, &Post{
		Title: "post-3",
	}).ID().Hex()

	r := gofight.New()

	// get posts in ascending order
	r.GET("/posts?sort=title").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post1+`",
						"attributes": {
							"title": "post-1",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post1+`/relationships/comments",
									"related": "/posts/`+post1+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post1+`/relationships/selections",
									"related": "/posts/`+post1+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post2+`",
						"attributes": {
							"title": "post-2",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post2+`/relationships/comments",
									"related": "/posts/`+post2+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post2+`/relationships/selections",
									"related": "/posts/`+post2+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post3+`",
						"attributes": {
							"title": "post-3",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post3+`/relationships/comments",
									"related": "/posts/`+post3+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post3+`/relationships/selections",
									"related": "/posts/`+post3+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})

	// get posts in descending order
	r.GET("/posts?sort=-title").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post3+`",
						"attributes": {
							"title": "post-3",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post3+`/relationships/comments",
									"related": "/posts/`+post3+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post3+`/relationships/selections",
									"related": "/posts/`+post3+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post2+`",
						"attributes": {
							"title": "post-2",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post2+`/relationships/comments",
									"related": "/posts/`+post2+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post2+`/relationships/selections",
									"related": "/posts/`+post2+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post1+`",
						"attributes": {
							"title": "post-1",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post1+`/relationships/comments",
									"related": "/posts/`+post1+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post1+`/relationships/selections",
									"related": "/posts/`+post1+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String())
		})
}

//func TestSparseFieldsets(t *testing.T) {
//	server, db := buildServer(&Resource{
//		Model: &Post{},
//	})
//
//	// create posts
//	post := saveModel(db, &Post{
//		Title: "Post 1",
//	}).ID().Hex()
//
//	r := gofight.New()
//
//	// get posts with single value filter
//	r.GET("/posts/"+post+"?fields[posts]=title").
//		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
//			assert.Equal(t, http.StatusOK, r.Code)
//			assert.JSONEq(t, `{
//				"data": {
//					"type": "posts",
//					"id": "`+post+`",
//					"attributes": {
//						"title": "Post 1"
//					},
//					"relationships": {
//						"comments": {
//							"links": {
//								"self": "/posts/`+post+`/relationships/comments",
//								"related": "/posts/`+post+`/comments"
//							}
//						},
//						"selections": {
//							"links": {
//								"self": "/posts/`+post+`/relationships/selections",
//								"related": "/posts/`+post+`/selections"
//							}
//						}
//					}
//				}
//			}`, r.Body.String())
//		})
//}

func TestHasManyRelationship(t *testing.T) {
	server, db := buildServer(&Resource{
		Model: &Post{},
	}, &Resource{
		Model: &Comment{},
	})

	// create existing post & comment
	existingPost := saveModel(db, &Post{
		Title: "Post 1",
	})
	saveModel(db, &Comment{
		Message: "Comment 1",
		PostID:  existingPost.ID(),
	})

	// create new post
	post := saveModel(db, &Post{
		Title: "Post 2",
	}).ID().Hex()

	r := gofight.New()

	// get single post
	r.GET("/posts/"+post).
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post+`",
					"attributes": {
						"title": "Post 2",
						"published": false,
						"text-body": ""
					},
					"relationships": {
						"comments": {
							"links": {
								"self": "/posts/`+post+`/relationships/comments",
								"related": "/posts/`+post+`/comments"
							}
						},
						"selections": {
							"links": {
								"self": "/posts/`+post+`/relationships/selections",
								"related": "/posts/`+post+`/selections"
							}
						}
					}
				},
				"links": {
					"self": "/posts/`+post+`"
				}
			}`, r.Body.String())
		})

	// get empty list of related comments
	r.GET("/posts/"+post+"/comments").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts/`+post+`/comments"
				}
			}`, r.Body.String())
		})

	var comment string

	// create related comment
	r.POST("/comments").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
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
							"id": "`+post+`"
						}
					}
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			comment = findLastModel(db, &Comment{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "comments",
					"id": "`+comment+`",
					"attributes": {
						"message": "Comment 2"
					},
					"relationships": {
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post+`"
							},
							"links": {
								"self": "/comments/`+comment+`/relationships/post",
								"related": "/comments/`+comment+`/post"
							}
						},
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment+`/relationships/parent",
								"related": "/comments/`+comment+`/parent"
							}
						}
					}
				},
				"links": {
					"self": "/comments/`+comment+`"
				}
			}`, r.Body.String())
		})

	// get list of related comments
	r.GET("/posts/"+post+"/comments").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "comments",
						"id": "`+comment+`",
						"attributes": {
							"message": "Comment 2"
						},
						"relationships": {
							"post": {
								"data": {
									"type": "posts",
									"id": "`+post+`"
								},
								"links": {
									"self": "/comments/`+comment+`/relationships/post",
									"related": "/comments/`+comment+`/post"
								}
							},
							"parent": {
								"data": null,
								"links": {
									"self": "/comments/`+comment+`/relationships/parent",
									"related": "/comments/`+comment+`/parent"
								}
							}
						}
					}
				],
				"links": {
					"self": "/posts/`+post+`/comments"
				}
			}`, r.Body.String())
		})

	// get only relationship links
	// TODO: We should see ids here.
	r.GET("/posts/"+post+"/relationships/comments").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"links": {
					"self": "/posts/`+post+`/relationships/comments",
					"related": "/posts/`+post+`/comments"
				}
			}`, r.Body.String())
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
	}).ID().Hex()
	post2 := saveModel(db, &Post{
		Title: "Post 2",
	}).ID().Hex()

	// create comment
	comment1 := saveModel(db, &Comment{
		Message: "Comment 1",
		PostID:  bson.ObjectIdHex(post1),
	}).ID().Hex()

	r := gofight.New()

	var comment2 string

	// create relating post
	r.POST("/comments").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
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
							"id": "`+post1+`"
						}
					},
					"parent": {
						"data": {
							"type": "comments",
							"id": "`+comment1+`"
						}
					}
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			comment2 = findLastModel(db, &Comment{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "comments",
					"id": "`+comment2+`",
					"attributes": {
						"message": "Comment 2"
					},
					"relationships": {
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post1+`"
							},
							"links": {
								"self": "/comments/`+comment2+`/relationships/post",
								"related": "/comments/`+comment2+`/post"
							}
						},
						"parent": {
							"data": {
								"type": "comments",
								"id": "`+comment1+`"
							},
							"links": {
								"self": "/comments/`+comment2+`/relationships/parent",
								"related": "/comments/`+comment2+`/parent"
							}
						}
					}
				},
				"links": {
					"self": "/comments/`+comment2+`"
				}
			}`, r.Body.String())
		})

	// get related post
	r.GET("/comments/"+comment2+"/post").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "Post 1",
						"published": false,
						"text-body": ""
					},
					"relationships": {
						"comments": {
							"links": {
								"self": "/posts/`+post1+`/relationships/comments",
								"related": "/posts/`+post1+`/comments"
							}
						},
						"selections": {
							"links": {
								"self": "/posts/`+post1+`/relationships/selections",
								"related": "/posts/`+post1+`/selections"
							}
						}
					}
				},
				"links": {
					"self": "/comments/`+comment2+`/post"
				}
			}`, r.Body.String())
		})

	// get related post id only
	r.GET("/comments/"+comment2+"/relationships/post").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`"
				},
				"links": {
					"self": "/comments/`+comment2+`/relationships/post",
					"related": "/comments/`+comment2+`/post"
				}
			}`, r.Body.String())
		})

	// replace relationship
	r.PATCH("/comments/"+comment2+"/relationships/post").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
		SetBody(`{
			"data": {
				"type": "comments",
				"id": "`+post2+`"
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// fetch replaced relationship
	r.GET("/comments/"+comment2+"/relationships/post").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post2+`"
				},
				"links": {
					"self": "/comments/`+comment2+`/relationships/post",
					"related": "/comments/`+comment2+`/post"
				}
			}`, r.Body.String())
		})

	// unset relationship
	r.PATCH("/comments/"+comment2+"/relationships/parent").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
		SetBody(`{
			"data": null
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// fetch unset relationship
	r.GET("/comments/"+comment2+"/relationships/parent").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": null,
				"links": {
					"self": "/comments/`+comment2+`/relationships/parent",
					"related": "/comments/`+comment2+`/parent"
				}
			}`, r.Body.String())
		})

	// fetch unset related resource
	r.GET("/comments/"+comment2+"/parent").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": null,
				"links": {
					"self": "/comments/`+comment2+`/parent"
				}
			}`, r.Body.String())
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
	}).ID().Hex()
	post2 := saveModel(db, &Post{
		Title: "Post 2",
	}).ID().Hex()
	post3 := saveModel(db, &Post{
		Title: "Post 3",
	}).ID().Hex()

	r := gofight.New()

	var selection string

	// create selection
	r.POST("/selections").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
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
								"id": "`+post1+`"
							},
							{
								"type": "posts",
								"id": "`+post2+`"
							}
						]
					}
				}
			}
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			selection = findLastModel(db, &Selection{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Code)
			assert.JSONEq(t, `{
				"data": {
					"type": "selections",
					"id": "`+selection+`",
					"attributes": {
						"name": "Selection 1"
					},
					"relationships": {
						"posts": {
							"data": [
								{
									"type": "posts",
									"id": "`+post1+`"
								},
								{
									"type": "posts",
									"id": "`+post2+`"
								}
							],
							"links": {
								"self": "/selections/`+selection+`/relationships/posts",
								"related": "/selections/`+selection+`/posts"
							}
						}
					}
				},
				"links": {
					"self": "/selections/`+selection+`"
				}
			}`, r.Body.String())
		})

	// get related post
	r.GET("/selections/"+selection+"/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post1+`",
						"attributes": {
							"title": "Post 1",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post1+`/relationships/comments",
									"related": "/posts/`+post1+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post1+`/relationships/selections",
									"related": "/posts/`+post1+`/selections"
								}
							}
						}
					},
					{
						"type": "posts",
						"id": "`+post2+`",
						"attributes": {
							"title": "Post 2",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"links": {
									"self": "/posts/`+post2+`/relationships/comments",
									"related": "/posts/`+post2+`/comments"
								}
							},
							"selections": {
								"links": {
									"self": "/posts/`+post2+`/relationships/selections",
									"related": "/posts/`+post2+`/selections"
								}
							}
						}
					}
				],
				"links": {
					"self": "/selections/`+selection+`/posts"
				}
			}`, r.Body.String())
		})

	// get related post ids only
	r.GET("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post1+`"
					},
					{
						"type": "posts",
						"id": "`+post2+`"
					}
				],
				"links": {
					"self": "/selections/`+selection+`/relationships/posts",
					"related": "/selections/`+selection+`/posts"
				}
			}`, r.Body.String())
		})

	// update relationship
	r.PATCH("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
		SetBody(`{
			"data": [
				{
					"type": "comments",
					"id": "`+post3+`"
				}
			]
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// get updated related post ids only
	r.GET("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post3+`"
					}
				],
				"links": {
					"self": "/selections/`+selection+`/relationships/posts",
					"related": "/selections/`+selection+`/posts"
				}
			}`, r.Body.String())
		})

	// add relationship
	r.POST("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
		SetBody(`{
			"data": [
				{
					"type": "comments",
					"id": "`+post1+`"
				}
			]
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// get related post ids only
	r.GET("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post3+`"
					},
					{
						"type": "posts",
						"id": "`+post1+`"
					}
				],
				"links": {
					"self": "/selections/`+selection+`/relationships/posts",
					"related": "/selections/`+selection+`/posts"
				}
			}`, r.Body.String())
		})

	// remove relationship
	r.DELETE("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept":       jsonapi.MediaType,
			"Content-Type": jsonapi.MediaType,
		}).
		SetBody(`{
			"data": [
				{
					"type": "comments",
					"id": "`+post3+`"
				},
				{
					"type": "comments",
					"id": "`+post1+`"
				}
			]
		}`).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusNoContent, r.Code)
			assert.Equal(t, "", r.Body.String())
		})

	// get empty related post ids list
	r.GET("/selections/"+selection+"/relationships/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data": [],
				"links": {
					"self": "/selections/`+selection+`/relationships/posts",
					"related": "/selections/`+selection+`/posts"
				}
			}`, r.Body.String())
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
	}).ID().Hex()

	// create selection
	selection := saveModel(db, &Selection{
		Name: "Selection 1",
	}).ID().Hex()

	r := gofight.New()

	// get related posts
	r.GET("/selections/"+selection+"/posts").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/selections/`+selection+`/posts"
				}
			}`, r.Body.String())
		})

	// get related selections
	r.GET("/posts/"+post+"/selections").
		SetHeader(gofight.H{
			"Accept": jsonapi.MediaType,
		}).
		Run(server, func(r gofight.HTTPResponse, rq gofight.HTTPRequest) {
			assert.Equal(t, http.StatusOK, r.Code)
			assert.JSONEq(t, `{
				"data":[],
				"links": {
					"self": "/posts/`+post+`/selections"
				}
			}`, r.Body.String())
		})
}
