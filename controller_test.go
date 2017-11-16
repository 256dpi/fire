package fire

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"gopkg.in/mgo.v2/bson"
)

func TestBasicOperations(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// get empty list of posts
	tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	var id string

	// create post
	tester.Request("POST", "posts", `{
		"data": {
			"type": "posts",
			"attributes": {
				"title": "Post 1"
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{})
		id = post.ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
						"data": [],
						"links": {
							"self": "/posts/`+id+`/relationships/comments",
							"related": "/posts/`+id+`/comments"
						}
					},
					"selections": {
						"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get list of posts
	tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+id+`/relationships/comments",
								"related": "/posts/`+id+`/comments"
							}
						},
						"selections": {
							"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// update post
	tester.Request("PATCH", "posts/"+id, `{
		"data": {
			"type": "posts",
			"id": "`+id+`",
			"attributes": {
				"text-body": "Post 1 Text"
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
						"data": [],
						"links": {
							"self": "/posts/`+id+`/relationships/comments",
							"related": "/posts/`+id+`/comments"
						}
					},
					"selections": {
						"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get single post
	tester.Request("GET", "posts/"+id, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
						"data": [],
						"links": {
							"self": "/posts/`+id+`/relationships/comments",
							"related": "/posts/`+id+`/comments"
						}
					},
					"selections": {
						"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// delete post
	tester.Request("DELETE", "posts/"+id, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty list of posts
	tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestFiltering(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
		Filters: []string{
			"title",
			"published",
		},
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create posts
	post1 := tester.Save(&postModel{
		Title:     "post-1",
		Published: true,
	}).ID().Hex()
	post2 := tester.Save(&postModel{
		Title:     "post-2",
		Published: false,
	}).ID().Hex()
	post3 := tester.Save(&postModel{
		Title:     "post-3",
		Published: true,
	}).ID().Hex()

	// create selection
	selection := tester.Save(&selectionModel{
		Posts: []bson.ObjectId{
			bson.ObjectIdHex(post1),
			bson.ObjectIdHex(post2),
			bson.ObjectIdHex(post3),
		},
	}).ID().Hex()

	// get posts with single value filter
	tester.Request("GET", "posts?filter[title]=post-1", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post1+`/relationships/comments",
								"related": "/posts/`+post1+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get posts with multi value filter
	tester.Request("GET", "posts?filter[title]=post-2,post-3", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post2+`/relationships/comments",
								"related": "/posts/`+post2+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
							"data": [],
							"links": {
								"self": "/posts/`+post3+`/relationships/comments",
								"related": "/posts/`+post3+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get posts with boolean
	tester.Request("GET", "posts?filter[published]=true", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post1+`/relationships/comments",
								"related": "/posts/`+post1+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
							"data": [],
							"links": {
								"self": "/posts/`+post3+`/relationships/comments",
								"related": "/posts/`+post3+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get posts with boolean
	tester.Request("GET", "posts?filter[published]=false", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post2+`/relationships/comments",
								"related": "/posts/`+post2+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get to-many posts with boolean
	tester.Request("GET", "selections/"+selection+"/posts?filter[published]=false", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post2+`/relationships/comments",
								"related": "/posts/`+post2+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestSorting(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
		Sorters: []string{
			"title",
		},
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create posts in random order
	post2 := tester.Save(&postModel{
		Title: "post-2",
	}).ID().Hex()
	post1 := tester.Save(&postModel{
		Title: "post-1",
	}).ID().Hex()
	post3 := tester.Save(&postModel{
		Title: "post-3",
	}).ID().Hex()

	// get posts in ascending order
	tester.Request("GET", "posts?sort=title", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post1+`/relationships/comments",
								"related": "/posts/`+post1+`/comments"
							}
						},
						"selections": {
							"data": [],
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
							"data": [],
							"links": {
								"self": "/posts/`+post2+`/relationships/comments",
								"related": "/posts/`+post2+`/comments"
							}
						},
						"selections": {
							"data": [],
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
							"data": [],
							"links": {
								"self": "/posts/`+post3+`/relationships/comments",
								"related": "/posts/`+post3+`/comments"
							}
						},
						"selections": {
							"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get posts in descending order
	tester.Request("GET", "posts?sort=-title", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post3+`/relationships/comments",
								"related": "/posts/`+post3+`/comments"
							}
						},
						"selections": {
							"data": [],
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
							"data": [],
							"links": {
								"self": "/posts/`+post2+`/relationships/comments",
								"related": "/posts/`+post2+`/comments"
							}
						},
						"selections": {
							"data": [],
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
							"data": [],
							"links": {
								"self": "/posts/`+post1+`/relationships/comments",
								"related": "/posts/`+post1+`/comments"
							}
						},
						"selections": {
							"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestSparseFields(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create posts
	post := tester.Save(&postModel{
		Title: "Post 1",
	}).ID().Hex()

	// get posts with single value filter
	tester.Request("GET", "posts/"+post+"?fields[posts]=title", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "posts",
				"id": "`+post+`",
				"attributes": {
					"title": "Post 1"
				},
				"relationships": {
					"comments": {
						"data": [],
						"links": {
							"self": "/posts/`+post+`/relationships/comments",
							"related": "/posts/`+post+`/comments"
						}
					},
					"selections": {
						"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestHasManyRelationship(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create existing post & comment
	existingPost := tester.Save(&postModel{
		Title: "Post 1",
	})
	tester.Save(&commentModel{
		Message: "Comment 1",
		Post:    existingPost.ID(),
	})

	// create new post
	post := tester.Save(&postModel{
		Title: "Post 2",
	}).ID().Hex()

	// get single post
	tester.Request("GET", "posts/"+post, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
						"data": [],
						"links": {
							"self": "/posts/`+post+`/relationships/comments",
							"related": "/posts/`+post+`/comments"
						}
					},
					"selections": {
						"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty list of related comments
	tester.Request("GET", "posts/"+post+"/comments", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/posts/`+post+`/comments"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	var comment string

	// create related comment
	tester.Request("POST", "comments", `{
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
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		comment = tester.FindLast(&commentModel{}).ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get list of related comments
	tester.Request("GET", "posts/"+post+"/comments", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get only relationship links
	tester.Request("GET", "posts/"+post+"/relationships/comments", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [
				{
					"type": "comments",
					"id": "`+comment+`"
				}
			],
			"links": {
				"self": "/posts/`+post+`/relationships/comments",
				"related": "/posts/`+post+`/comments"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestToOneRelationship(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create posts
	post1 := tester.Save(&postModel{
		Title: "Post 1",
	}).ID().Hex()
	post2 := tester.Save(&postModel{
		Title: "Post 2",
	}).ID().Hex()

	// create comment
	comment1 := tester.Save(&commentModel{
		Message: "Comment 1",
		Post:    bson.ObjectIdHex(post1),
	}).ID().Hex()

	var comment2 string

	// create relating post
	tester.Request("POST", "comments", `{
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
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		comment2 = tester.FindLast(&commentModel{}).ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related post
	tester.Request("GET", "comments/"+comment2+"/post", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
						"data": [
							{
								"type": "comments",
								"id": "`+comment1+`"
							},
							{
								"type": "comments",
								"id": "`+comment2+`"
							}
						],
						"links": {
							"self": "/posts/`+post1+`/relationships/comments",
							"related": "/posts/`+post1+`/comments"
						}
					},
					"selections": {
						"data": [],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related post id only
	tester.Request("GET", "comments/"+comment2+"/relationships/post", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`"
			},
			"links": {
				"self": "/comments/`+comment2+`/relationships/post",
				"related": "/comments/`+comment2+`/post"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// replace relationship
	tester.Request("PATCH", "comments/"+comment2+"/relationships/post", `{
		"data": {
			"type": "comments",
			"id": "`+post2+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// fetch replaced relationship
	tester.Request("GET", "comments/"+comment2+"/relationships/post", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "posts",
				"id": "`+post2+`"
			},
			"links": {
				"self": "/comments/`+comment2+`/relationships/post",
				"related": "/comments/`+comment2+`/post"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// unset relationship
	tester.Request("PATCH", "comments/"+comment2+"/relationships/parent", `{
			"data": null
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// fetch unset relationship
	tester.Request("GET", "comments/"+comment2+"/relationships/parent", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": null,
			"links": {
				"self": "/comments/`+comment2+`/relationships/parent",
				"related": "/comments/`+comment2+`/parent"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// fetch unset related resource
	tester.Request("GET", "comments/"+comment2+"/parent", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": null,
			"links": {
				"self": "/comments/`+comment2+`/parent"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestToManyRelationship(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create posts
	post1 := tester.Save(&postModel{
		Title: "Post 1",
	}).ID().Hex()
	post2 := tester.Save(&postModel{
		Title: "Post 2",
	}).ID().Hex()
	post3 := tester.Save(&postModel{
		Title: "Post 3",
	}).ID().Hex()

	var selection string

	// create selection
	tester.Request("POST", "selections", `{
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
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		selection = tester.FindLast(&selectionModel{}).ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related post
	tester.Request("GET", "selections/"+selection+"/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
							"data": [],
							"links": {
								"self": "/posts/`+post1+`/relationships/comments",
								"related": "/posts/`+post1+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
							"data": [],
							"links": {
								"self": "/posts/`+post2+`/relationships/comments",
								"related": "/posts/`+post2+`/comments"
							}
						},
						"selections": {
							"data": [
								{
									"type": "selections",
									"id": "`+selection+`"
								}
							],
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related post ids only
	tester.Request("GET", "selections/"+selection+"/relationships/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// update relationship
	tester.Request("PATCH", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "comments",
				"id": "`+post3+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get updated related post ids only
	tester.Request("GET", "selections/"+selection+"/relationships/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// add relationship
	tester.Request("POST", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "comments",
				"id": "`+post1+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related post ids only
	tester.Request("GET", "selections/"+selection+"/relationships/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
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
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// remove relationship
	tester.Request("DELETE", "selections/"+selection+"/relationships/posts", `{
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
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty related post ids list
	tester.Request("GET", "selections/"+selection+"/relationships/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [],
			"links": {
				"self": "/selections/`+selection+`/relationships/posts",
				"related": "/selections/`+selection+`/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestEmptyToManyRelationship(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create posts
	post := tester.Save(&postModel{
		Title: "Post 1",
	}).ID().Hex()

	// create selection
	selection := tester.Save(&selectionModel{
		Name: "Selection 1",
	}).ID().Hex()

	// get related posts
	tester.Request("GET", "selections/"+selection+"/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/selections/`+selection+`/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related selections
	tester.Request("GET", "posts/"+post+"/selections", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/posts/`+post+`/selections"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestNoList(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model:  &commentModel{},
		Store:  testStore,
		NoList: true,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// attempt list comments
	tester.Request("GET", "comments", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusMethodNotAllowed, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Contains(t, r.Body.String(), "Listing is disabled for this resource.", tester.DebugRequest(rq, r))
	})
}

func TestPagination(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create some posts
	for i := 0; i < 10; i++ {
		tester.Save(&postModel{
			Title: fmt.Sprintf("Post %d", i+1),
		})
	}

	// get first page of posts
	tester.Request("GET", "posts?page[number]=1&page[size]=5", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Post 1", list[0].Get("attributes.title").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/posts?page[number]=1&page[size]=5",
			"first": "/posts?page[number]=1&page[size]=5",
			"last": "/posts?page[number]=2&page[size]=5",
			"next": "/posts?page[number]=2&page[size]=5"
		}`, links)
	})

	// get second page of posts
	tester.Request("GET", "posts?page[number]=2&page[size]=5", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Post 6", list[0].Get("attributes.title").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/posts?page[number]=2&page[size]=5",
			"first": "/posts?page[number]=1&page[size]=5",
			"last": "/posts?page[number]=2&page[size]=5",
			"prev": "/posts?page[number]=1&page[size]=5"
		}`, links)
	})
}

func TestPaginationToMany(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// prepare ids
	var ids []bson.ObjectId

	// create some posts
	for i := 0; i < 10; i++ {
		ids = append(ids, tester.Save(&postModel{
			Title: fmt.Sprintf("Post %d", i+1),
		}).ID())
	}

	// create selection
	selection := tester.Save(&selectionModel{
		Posts: ids,
	}).ID().Hex()

	// get first page of posts
	tester.Request("GET", "selections/"+selection+"/posts?page[number]=1&page[size]=5", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Post 1", list[0].Get("attributes.title").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/selections/`+selection+`/posts?page[number]=1&page[size]=5",
			"first": "/selections/`+selection+`/posts?page[number]=1&page[size]=5",
			"last": "/selections/`+selection+`/posts?page[number]=2&page[size]=5",
			"next": "/selections/`+selection+`/posts?page[number]=2&page[size]=5"
		}`, links)
	})

	// get second page of posts
	tester.Request("GET", "selections/"+selection+"/posts?page[number]=2&page[size]=5", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Post 6", list[0].Get("attributes.title").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/selections/`+selection+`/posts?page[number]=2&page[size]=5",
			"first": "/selections/`+selection+`/posts?page[number]=1&page[size]=5",
			"last": "/selections/`+selection+`/posts?page[number]=2&page[size]=5",
			"prev": "/selections/`+selection+`/posts?page[number]=1&page[size]=5"
		}`, links)
	})
}

func TestPaginationHasMany(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model: &postModel{},
		Store: testStore,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create post
	post := tester.Save(&postModel{
		Title: "Post",
	}).ID()

	// create some comments
	for i := 0; i < 10; i++ {
		tester.Save(&commentModel{
			Message: fmt.Sprintf("Comment %d", i+1),
			Post:    post,
		})
	}

	// get first page of posts
	tester.Request("GET", "posts/"+post.Hex()+"/comments?page[number]=1&page[size]=5", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Comment 1", list[0].Get("attributes.message").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/posts/`+post.Hex()+`/comments?page[number]=1&page[size]=5",
			"first": "/posts/`+post.Hex()+`/comments?page[number]=1&page[size]=5",
			"last": "/posts/`+post.Hex()+`/comments?page[number]=2&page[size]=5",
			"next": "/posts/`+post.Hex()+`/comments?page[number]=2&page[size]=5"
		}`, links)
	})

	// get second page of posts
	tester.Request("GET", "posts/"+post.Hex()+"/comments?page[number]=2&page[size]=5", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Comment 6", list[0].Get("attributes.message").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/posts/`+post.Hex()+`/comments?page[number]=2&page[size]=5",
			"first": "/posts/`+post.Hex()+`/comments?page[number]=1&page[size]=5",
			"last": "/posts/`+post.Hex()+`/comments?page[number]=2&page[size]=5",
			"prev": "/posts/`+post.Hex()+`/comments?page[number]=1&page[size]=5"
		}`, links)
	})
}

func TestForcedPagination(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model:     &postModel{},
		Store:     testStore,
		ListLimit: 5,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create some posts
	for i := 0; i < 10; i++ {
		tester.Save(&postModel{
			Title: fmt.Sprintf("Post %d", i+1),
		})
	}

	// get first page of posts
	tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Post 1", list[0].Get("attributes.title").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/posts?page[number]=1&page[size]=5",
			"first": "/posts?page[number]=1&page[size]=5",
			"last": "/posts?page[number]=2&page[size]=5",
			"next": "/posts?page[number]=2&page[size]=5"
		}`, links)
	})
}

func TestEnforcedListLimit(t *testing.T) {
	tester.Clean()

	tester.Handler = buildHandler(&Controller{
		Model:     &postModel{},
		Store:     testStore,
		ListLimit: 5,
	}, &Controller{
		Model: &commentModel{},
		Store: testStore,
	}, &Controller{
		Model: &selectionModel{},
		Store: testStore,
	})

	// create some posts
	for i := 0; i < 10; i++ {
		tester.Save(&postModel{
			Title: fmt.Sprintf("Post %d", i+1),
		})
	}

	// get first page of posts
	tester.Request("GET", "posts?page[number]=1&page[size]=7", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		list := gjson.Get(r.Body.String(), "data").Array()
		links := gjson.Get(r.Body.String(), "links").Raw

		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, 5, len(list), tester.DebugRequest(rq, r))
		assert.Equal(t, "Post 1", list[0].Get("attributes.title").String(), tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"self": "/posts?page[number]=1&page[size]=5",
			"first": "/posts?page[number]=1&page[size]=5",
			"last": "/posts?page[number]=2&page[size]=5",
			"next": "/posts?page[number]=2&page[size]=5"
		}`, links)
	})
}
