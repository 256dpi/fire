package fire

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/globalsign/mgo/bson"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestBasicOperations(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
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

	// attempt to create post with missing document
	tester.Request("POST", "posts", `{}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "missing document"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to create post with invalid type
	tester.Request("POST", "posts", `{
		"data": {
			"type": "foo"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to create post with invalid id
	tester.Request("POST", "posts", `{
		"data": {
			"type": "posts",
			"id": "`+bson.NewObjectId().Hex()+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "unnecessary resource id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to create post with invalid attribute
	tester.Request("POST", "posts", `{
		"data": {
			"type": "posts",
			"attributes": {
				"foo": "bar"
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid attribute"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+id+`/relationships/note",
							"related": "/posts/`+id+`/note"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+id+`/relationships/note",
								"related": "/posts/`+id+`/note"
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

	// attempt to update post with missing document
	tester.Request("PATCH", "posts/"+id, `{}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{})
		id = post.ID().Hex()

		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "missing document"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to update post with invalid type
	tester.Request("PATCH", "posts/"+id, `{
		"data": {
			"type": "foo",
			"id": "`+id+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{})
		id = post.ID().Hex()

		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to update post with invalid id
	tester.Request("PATCH", "posts/"+id, `{
		"data": {
			"type": "posts",
			"id": "`+bson.NewObjectId().Hex()+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{})
		id = post.ID().Hex()

		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource id mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to update post with invalid id
	tester.Request("PATCH", "posts/foo", `{
		"data": {
			"type": "posts",
			"id": "`+id+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{})
		id = post.ID().Hex()

		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid resource id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to update post with invalid attribute
	tester.Request("PATCH", "posts/"+id, `{
		"data": {
			"type": "posts",
			"id": "`+id+`",
			"attributes": {
				"foo": "bar"
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{})
		id = post.ID().Hex()

		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid attribute"
			}]
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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+id+`/relationships/note",
							"related": "/posts/`+id+`/note"
						}
					}
				}
			},
			"links": {
				"self": "/posts/`+id+`"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to get single post with invalid id
	tester.Request("GET", "posts/foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid resource id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to get not existing post
	tester.Request("GET", "posts/"+bson.NewObjectId().Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNotFound, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "404",
				"title": "Not Found",
				"detail": "resource not found"
			}]
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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+id+`/relationships/note",
							"related": "/posts/`+id+`/note"
						}
					}
				}
			},
			"links": {
				"self": "/posts/`+id+`"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt delete post with invalid id
	tester.Request("DELETE", "posts/foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid resource id"
			}]
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

func TestHasOneRelationship(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+post+`/relationships/note",
							"related": "/posts/`+post+`/note"
						}
					}
				}
			},
			"links": {
				"self": "/posts/`+post+`"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get invalid relation
	tester.Request("GET", "posts/"+post+"/foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get invalid relationship
	tester.Request("GET", "posts/"+post+"/relationships/foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty unset related note
	tester.Request("GET", "posts/"+post+"/note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": null,
			"links": {
				"self": "/posts/`+post+`/note"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to create related note with invalid relationship
	tester.Request("POST", "notes", `{
		"data": {
			"type": "notes",
			"attributes": {
				"title": "Note 2"
			},
			"relationships": {
				"foo": {
					"data": {
						"type": "foo",
						"id": "`+post+`"
					}
				}
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to create related note with invalid type
	tester.Request("POST", "notes", `{
		"data": {
			"type": "notes",
			"attributes": {
				"title": "Note 2"
			},
			"relationships": {
				"post": {
					"data": {
						"type": "foo",
						"id": "`+post+`"
					}
				}
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	var note string

	// create related note
	tester.Request("POST", "notes", `{
		"data": {
			"type": "notes",
			"attributes": {
				"title": "Note 2"
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
		note = tester.FindLast(&noteModel{}).ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "notes",
				"id": "`+note+`",
				"attributes": {
					"title": "Note 2"
				},
				"relationships": {
					"post": {
						"data": {
							"type": "posts",
							"id": "`+post+`"
						},
						"links": {
							"self": "/notes/`+note+`/relationships/post",
							"related": "/notes/`+note+`/post"
						}
					}
				}
			},
			"links": {
				"self": "/notes/`+note+`"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get related note
	tester.Request("GET", "posts/"+post+"/note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "notes",
				"id": "`+note+`",
				"attributes": {
					"title": "Note 2"
				},
				"relationships": {
					"post": {
						"data": {
							"type": "posts",
							"id": "`+post+`"
						},
						"links": {
							"self": "/notes/`+note+`/relationships/post",
							"related": "/notes/`+note+`/post"
						}
					}
				}
			},
			"links": {
				"self": "/posts/`+post+`/note"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get note relationship
	tester.Request("GET", "posts/"+post+"/relationships/note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "notes",
				"id": "`+note+`"
			},
			"links": {
				"self": "/posts/`+post+`/relationships/note",
				"related": "/posts/`+post+`/note"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestHasManyRelationship(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+post+`/relationships/note",
							"related": "/posts/`+post+`/note"
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

	// attempt to create related comment with invalid type
	tester.Request("POST", "comments", `{
		"data": {
			"type": "comments",
			"attributes": {
				"message": "Comment 2"
			},
			"relationships": {
				"post": {
					"data": {
						"type": "foo",
						"id": "`+post+`"
					}
				}
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
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

	// get comments relationship
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

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
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

	// create relating comment
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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+post1+`/relationships/note",
							"related": "/posts/`+post1+`/note"
						}
					}
				}
			},
			"links": {
				"self": "/comments/`+comment2+`/post"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get post relationship
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

	// attempt to replace invalid relationship
	tester.Request("PATCH", "comments/"+comment2+"/relationships/foo", `{
		"data": {
			"type": "posts",
			"id": "`+post2+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to replace post relationship with invalid type
	tester.Request("PATCH", "comments/"+comment2+"/relationships/post", `{
		"data": {
			"type": "foo",
			"id": "`+post2+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to replace post relationship with invalid id
	tester.Request("PATCH", "comments/"+comment2+"/relationships/post", `{
		"data": {
			"type": "posts",
			"id": "foo"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// replace post relationship
	tester.Request("PATCH", "comments/"+comment2+"/relationships/post", `{
		"data": {
			"type": "posts",
			"id": "`+post2+`"
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
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

	// get replaced post relationship
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

	// get existing parent relationship
	tester.Request("GET", "comments/"+comment2+"/relationships/parent", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "comments",
				"id": "`+comment1+`"
			},
			"links": {
				"self": "/comments/`+comment2+`/relationships/parent",
				"related": "/comments/`+comment2+`/parent"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get existing related resource
	tester.Request("GET", "comments/"+comment2+"/parent", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "comments",
				"id": "`+comment1+`",
				"attributes": {
					"message": "Comment 1"
				},
				"relationships": {
					"parent": {
						"data": null,
						"links": {
							"self": "/comments/`+comment1+`/relationships/parent",
							"related": "/comments/`+comment1+`/parent"
						}
					},
					"post": {
						"data": {
							"type": "posts",
							"id": "`+post1+`"
						},
						"links": {
							"self": "/comments/`+comment1+`/relationships/post",
							"related": "/comments/`+comment1+`/post"
						}
					}
				}
			},
			"links": {
				"self": "/comments/`+comment2+`/parent"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// unset parent relationship
	tester.Request("PATCH", "comments/"+comment2+"/relationships/parent", `{
			"data": null
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": null,
			"links": {
				"self": "/comments/`+comment2+`/relationships/parent",
				"related": "/comments/`+comment2+`/parent"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// fetch unset parent relationship
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

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post1+`/relationships/note",
								"related": "/posts/`+post1+`/note"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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

	// get posts relationship
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

	// attempt to replace posts relationship with invalid type
	tester.Request("PATCH", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "foo",
				"id": "`+post3+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to replace posts relationship with invalid id
	tester.Request("PATCH", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "foo"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// replace posts relationship
	tester.Request("PATCH", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post3+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
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

	// get updated posts relationship
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

	// attempt to add to invalid relationship
	tester.Request("POST", "selections/"+selection+"/relationships/foo", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post1+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to add to posts relationship with invalid type
	tester.Request("POST", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "foo",
				"id": "`+post1+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to add to posts relationship with invalid id
	tester.Request("POST", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "foo"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// add to posts relationship
	tester.Request("POST", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post1+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
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

	// add existing id to posts relationship
	tester.Request("POST", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post1+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
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

	// get posts relationship
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

	// attempt to remove from invalid relationship
	tester.Request("DELETE", "selections/"+selection+"/relationships/foo", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post3+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to remove from posts relationships with invalid type
	tester.Request("DELETE", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "foo",
				"id": "`+post3+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "resource type mismatch"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to remove from posts relationships with invalid id
	tester.Request("DELETE", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "foo"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid relationship id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// remove from posts relationships
	tester.Request("DELETE", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post3+`"
			},
			{
				"type": "posts",
				"id": "`+post1+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [],
			"links": {
				"self": "/selections/`+selection+`/relationships/posts",
				"related": "/selections/`+selection+`/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty posts relationship
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

	// get empty list of related posts
	tester.Request("GET", "selections/"+selection+"/posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/selections/`+selection+`/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty list of related selections
	tester.Request("GET", "posts/"+post1+"/selections", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data":[],
			"links": {
				"self": "/posts/`+post1+`/selections"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestFiltering(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model:   &postModel{},
		Store:   tester.Store,
		Filters: []string{"Title", "Published"},
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model:   &selectionModel{},
		Store:   tester.Store,
		Filters: []string{"Posts"},
	}, &Controller{
		Model:   &noteModel{},
		Store:   tester.Store,
		Filters: []string{"Post"},
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

	// create selections
	selection := tester.Save(&selectionModel{
		Name: "selection-1",
		Posts: []bson.ObjectId{
			bson.ObjectIdHex(post1),
			bson.ObjectIdHex(post2),
			bson.ObjectIdHex(post3),
		},
	}).ID().Hex()
	tester.Save(&selectionModel{
		Name: "selection-2",
	})

	// create notes
	note := tester.Save(&noteModel{
		Title: "note-1",
		Post:  bson.ObjectIdHex(post1),
	}).ID().Hex()
	tester.Save(&noteModel{
		Title: "note-2",
		Post:  bson.NewObjectId(),
	})

	// test invalid filter
	tester.Request("GET", "posts?filter[foo]=bar", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid filter \"foo\""
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// test not supported filter
	tester.Request("GET", "posts?filter[text-body]=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid filter \"text-body\""
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

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
						},
						"note": {
							"data": {
								"type": "notes",
								"id": "`+note+`"
							},
							"links": {
								"self": "/posts/`+post1+`/relationships/note",
								"related": "/posts/`+post1+`/note"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post3+`/relationships/note",
								"related": "/posts/`+post3+`/note"
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
						},
						"note": {
							"data": {
								"type": "notes",
								"id": "`+note+`"
							},
							"links": {
								"self": "/posts/`+post1+`/relationships/note",
								"related": "/posts/`+post1+`/note"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post3+`/relationships/note",
								"related": "/posts/`+post3+`/note"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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

	// test not supported relationship filter
	tester.Request("GET", "comments?filter[post]=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid filter \"post\""
			}]
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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

	// test invalid relationship filter id
	tester.Request("GET", "notes?filter[post]=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship filter value is not an object id"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// filter notes with to-one relationship filter
	tester.Request("GET", "notes?filter[post]="+post1, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
  			"data": [
    			{
      				"type": "notes",
      				"id": "`+note+`",
      				"attributes": {
        				"title": "note-1"
      				},
      				"relationships": {
        				"post": {
          					"data": {
            					"type": "posts",
            					"id": "`+post1+`"
          					},
          					"links": {
            					"self": "/notes/`+note+`/relationships/post",
            					"related": "/notes/`+note+`/post"
          					}
			        	}
      				}
    			}
  			],
  			"links": {
    			"self": "/notes"
  			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// filter selections with to-many relationship filter
	tester.Request("GET", "selections?filter[posts]="+post1, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [
				{
			  		"type": "selections",
			  		"id": "`+selection+`",
			  		"attributes": {
						"name": "selection-1"
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
								},
								{
									"type": "posts",
									"id": "`+post3+`"
								}
							],
							"links": {
								"self": "/selections/`+selection+`/relationships/posts",
								"related": "/selections/`+selection+`/posts"
							}
						}
					}
				}
		  	],
		  	"links": {
				"self": "/selections"
		  	}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// TODO: Test array filtering.
}

func TestSorting(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model:   &postModel{},
		Store:   tester.Store,
		Sorters: []string{"Title", "TextBody"},
	}, &Controller{
		Model:   &commentModel{},
		Store:   tester.Store,
		Sorters: []string{"Message"},
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// create posts in random order
	post2 := tester.Save(&postModel{
		Title:    "post-2",
		TextBody: "body-2",
	}).ID().Hex()
	post1 := tester.Save(&postModel{
		Title:    "post-1",
		TextBody: "body-1",
	}).ID().Hex()
	post3 := tester.Save(&postModel{
		Title:    "post-3",
		TextBody: "body-3",
	}).ID().Hex()

	// test invalid sorter
	tester.Request("GET", "posts?sort=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid sorter \"foo\""
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// test invalid sorter
	tester.Request("GET", "posts?sort=published", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "400",
				"title": "Bad Request",
				"detail": "unsupported sorter \"published\""
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

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
						"text-body": "body-1"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post1+`/relationships/note",
								"related": "/posts/`+post1+`/note"
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
						"text-body": "body-2"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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
						"text-body": "body-3"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post3+`/relationships/note",
								"related": "/posts/`+post3+`/note"
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
						"text-body": "body-3"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post3+`/relationships/note",
								"related": "/posts/`+post3+`/note"
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
						"text-body": "body-2"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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
						"text-body": "body-1"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post1+`/relationships/note",
								"related": "/posts/`+post1+`/note"
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

	// get posts in ascending order
	tester.Request("GET", "posts?sort=text-body", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [
				{
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": false,
						"text-body": "body-1"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post1+`/relationships/note",
								"related": "/posts/`+post1+`/note"
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
						"text-body": "body-2"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post2+`/relationships/note",
								"related": "/posts/`+post2+`/note"
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
						"text-body": "body-3"
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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+post3+`/relationships/note",
								"related": "/posts/`+post3+`/note"
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

	// create post
	post := tester.Save(&postModel{
		Title: "Post",
	}).ID()

	// create some comments
	comment1 := tester.Save(&commentModel{
		Message: "Comment 1",
		Post:    post,
	}).ID().Hex()
	comment2 := tester.Save(&commentModel{
		Message: "Comment 2",
		Post:    post,
	}).ID().Hex()
	comment3 := tester.Save(&commentModel{
		Message: "Comment 3",
		Post:    post,
	}).ID().Hex()

	// get first page of comments
	tester.Request("GET", "posts/"+post.Hex()+"/comments?sort=message", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [
				{
					"type": "comments",
					"id": "`+comment1+`",
					"attributes": {
						"message": "Comment 1"
					},
					"relationships": {
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment1+`/relationships/parent",
								"related": "/comments/`+comment1+`/parent"
							}
						},
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post.Hex()+`"
							},
							"links": {
								"self": "/comments/`+comment1+`/relationships/post",
								"related": "/comments/`+comment1+`/post"
							}
						}
					}
				},
				{
					"type": "comments",
					"id": "`+comment2+`",
					"attributes": {
						"message": "Comment 2"
					},
					"relationships": {
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment2+`/relationships/parent",
								"related": "/comments/`+comment2+`/parent"
							}
						},
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post.Hex()+`"
							},
							"links": {
								"self": "/comments/`+comment2+`/relationships/post",
								"related": "/comments/`+comment2+`/post"
							}
						}
					}
				},
				{
					"type": "comments",
					"id": "`+comment3+`",
					"attributes": {
						"message": "Comment 3"
					},
					"relationships": {
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment3+`/relationships/parent",
								"related": "/comments/`+comment3+`/parent"
							}
						},
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post.Hex()+`"
							},
							"links": {
								"self": "/comments/`+comment3+`/relationships/post",
								"related": "/comments/`+comment3+`/post"
							}
						}
					}
				}
			],
			"links": {
				"self": "/posts/`+post.Hex()+`/comments"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get second page of comments
	tester.Request("GET", "posts/"+post.Hex()+"/comments?sort=-message", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [
				{
					"type": "comments",
					"id": "`+comment3+`",
					"attributes": {
						"message": "Comment 3"
					},
					"relationships": {
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment3+`/relationships/parent",
								"related": "/comments/`+comment3+`/parent"
							}
						},
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post.Hex()+`"
							},
							"links": {
								"self": "/comments/`+comment3+`/relationships/post",
								"related": "/comments/`+comment3+`/post"
							}
						}
					}
				},
				{
					"type": "comments",
					"id": "`+comment2+`",
					"attributes": {
						"message": "Comment 2"
					},
					"relationships": {
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment2+`/relationships/parent",
								"related": "/comments/`+comment2+`/parent"
							}
						},
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post.Hex()+`"
							},
							"links": {
								"self": "/comments/`+comment2+`/relationships/post",
								"related": "/comments/`+comment2+`/post"
							}
						}
					}
				},
				{
					"type": "comments",
					"id": "`+comment1+`",
					"attributes": {
						"message": "Comment 1"
					},
					"relationships": {
						"parent": {
							"data": null,
							"links": {
								"self": "/comments/`+comment1+`/relationships/parent",
								"related": "/comments/`+comment1+`/parent"
							}
						},
						"post": {
							"data": {
								"type": "posts",
								"id": "`+post.Hex()+`"
							},
							"links": {
								"self": "/comments/`+comment1+`/relationships/post",
								"related": "/comments/`+comment1+`/post"
							}
						}
					}
				}
			],
			"links": {
				"self": "/posts/`+post.Hex()+`/comments"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestSparseFields(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// create post
	post := tester.Save(&postModel{
		Title: "Post 1",
	}).ID()

	// get posts with invalid filter
	tester.Request("GET", "posts/"+post.Hex()+"?fields[posts]=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "invalid sparse field \"foo\""
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get posts with single value filter
	tester.Request("GET", "posts/"+post.Hex()+"?fields[posts]=title&fields[posts]=note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "posts",
				"id": "`+post.Hex()+`",
				"attributes": {
					"title": "Post 1"
				},
				"relationships": {
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+post.Hex()+`/relationships/note",
							"related": "/posts/`+post.Hex()+`/note"
						}
					}
				}
			},
			"links": {
				"self": "/posts/`+post.Hex()+`"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// create note
	note := tester.Save(&noteModel{
		Title: "Note 1",
		Post:  post,
	}).ID()

	// get related note
	tester.Request("GET", "/posts/"+post.Hex()+"/note?fields[notes]=post", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "notes",
				"id": "`+note.Hex()+`",
				"relationships": {
					"post": {
						"data": {
							"type": "posts",
							"id": "`+post.Hex()+`"
						},
						"links": {
							"self": "/notes/`+note.Hex()+`/relationships/post",
							"related": "/notes/`+note.Hex()+`/post"
						}
					}
				}
			},
			"links": {
				"self": "/posts/`+post.Hex()+`/note"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestReadableFields(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
		Authorizers: L{
			C("TestReadableFields", All(), func(ctx *Context) error {
				ctx.ReadableFields = []string{"Published"}
				return nil
			}),
		},
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
		Authorizers: L{
			C("TestReadableFields", All(), func(ctx *Context) error {
				ctx.ReadableFields = []string{}
				return nil
			}),
		},
	})

	// create post
	post := tester.Save(&postModel{
		Title:     "post-1",
		Published: true,
	}).ID()

	// get posts with single value filter
	tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": [
				{
					"type": "posts",
					"id": "`+post.Hex()+`",
					"attributes": {
						"published": true
					}
				}
			],
			"links": {
				"self": "/posts"
			}
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to get relationship
	tester.Request("GET", "/posts/"+post.Hex()+"/relationships/note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship is not readable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to get related note
	tester.Request("GET", "/posts/"+post.Hex()+"/note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship is not readable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestWritableFields(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
		Authorizers: L{
			C("TestWritableFields", All(), func(ctx *Context) error {
				ctx.WritableFields = []string{"Title"}
				return nil
			}),
		},
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
		Authorizers: L{
			C("TestWritableFields", All(), func(ctx *Context) error {
				ctx.WritableFields = []string{}
				return nil
			}),
		},
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// attempt to create post with protected field
	tester.Request("POST", "posts", `{
		"data": {
			"type": "posts",
			"attributes": {
				"title": "Post 1",
				"published": true
			},
			"relationships": {}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "attribute is not writable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to create selection with protected relationship
	tester.Request("POST", "selections", `{
		"data": {
			"type": "selections",
			"attributes": {},
			"relationships": {
				"posts": {
					"data": [{
						"type": "posts",
						"id": "`+bson.NewObjectId().Hex()+`"
					}]
				}
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship is not writable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	post := bson.NewObjectId()

	selection := tester.Save(&selectionModel{
		Posts: []bson.ObjectId{post},
	}).ID().Hex()

	// attempt to update posts relationship
	tester.Request("PATCH", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+bson.NewObjectId().Hex()+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship is not writable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to add to posts relationship
	tester.Request("POST", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+bson.NewObjectId().Hex()+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship is not writable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// attempt to remove from posts relationship
	tester.Request("DELETE", "selections/"+selection+"/relationships/posts", `{
		"data": [
			{
				"type": "posts",
				"id": "`+post.Hex()+`"
			}
		]
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [{
				"status": "400",
				"title": "Bad Request",
				"detail": "relationship is not writable"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestSoftProtection(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
		Authorizers: L{
			C("TestWritableFields", All(), func(ctx *Context) error {
				ctx.WritableFields = []string{"Title"}
				return nil
			}),
		},
		SoftProtection: true,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
		Authorizers: L{
			C("TestWritableFields", All(), func(ctx *Context) error {
				ctx.WritableFields = []string{}
				return nil
			}),
		},
		SoftProtection: true,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// attempt to create post with protected field
	tester.Request("POST", "posts", `{
		"data": {
			"type": "posts",
			"attributes": {
				"title": "Post 1",
				"published": true
			},
			"relationships": {}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		post := tester.FindLast(&postModel{}).ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "posts",
				"id": "`+post+`",
				"attributes": {
					"title": "Post 1",
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
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+post+`/relationships/note",
							"related": "/posts/`+post+`/note"
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

	// attempt to create selection with protected relationship
	tester.Request("POST", "selections", `{
		"data": {
			"type": "selections",
			"attributes": {},
			"relationships": {
				"posts": {
					"data": [{
						"type": "posts",
						"id": "`+bson.NewObjectId().Hex()+`"
					}]
				}
			}
		}
	}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
		selection := tester.FindLast(&selectionModel{}).ID().Hex()

		assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"data": {
				"type": "selections",
				"id": "`+selection+`",
				"attributes": {
					"name": ""
				},
				"relationships": {
					"posts": {
						"data": [],
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
}

func TestNoList(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model:  &commentModel{},
		Store:  tester.Store,
		NoList: true,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// attempt list comments
	tester.Request("GET", "comments", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusMethodNotAllowed, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors":[{
				"status": "405",
				"title": "Method Not Allowed",
				"detail": "listing is disabled for this resource"
			}]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestPagination(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// prepare ids
	var ids []bson.ObjectId

	// create some posts
	for i := 0; i < 10; i++ {
		ids = append(ids, tester.Save(&postModel{
			Title: fmt.Sprintf("Post %d", i+1),
		}).ID())
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

	// get first page of comments
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

	// get second page of comments
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

	tester.Assign("", &Controller{
		Model:     &postModel{},
		Store:     tester.Store,
		ListLimit: 5,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
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

func TestCollectionActions(t *testing.T) {
	tester.Clean()

	tester.Assign("api", &Controller{
		Model: &postModel{},
		Store: tester.Store,
		CollectionActions: M{
			"bytes": {
				Methods: []string{"POST"},
				Callback: C("bytes", All(), func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Equal(t, []byte("PAYLOAD"), bytes)

					_, err = ctx.ResponseWriter.Write([]byte("RESPONSE"))
					assert.NoError(t, err)

					return nil
				}),
			},
			"empty": {
				Methods: []string{"POST"},
				Callback: C("empty", All(), func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Empty(t, bytes)

					return nil
				}),
			},
			"error": {
				Methods:   []string{"POST"},
				BodyLimit: 3,
				Callback: C("error", All(), func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.Error(t, err)
					assert.Equal(t, []byte("PAY"), bytes)

					ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)

					return nil
				}),
			},
		},
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	// get byte response
	tester.Header["Content-Type"] = "text/plain"
	tester.Header["Accept"] = "text/plain"
	tester.Request("POST", "posts/bytes", "PAYLOAD", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "text/plain; charset=utf-8", r.Result().Header.Get("Content-Type"), tester.DebugRequest(rq, r))
		assert.Equal(t, "RESPONSE", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty response
	tester.Header["Content-Type"] = ""
	tester.Header["Accept"] = ""
	tester.Request("POST", "posts/empty", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Empty(t, r.Result().Header.Get("Content-Type"), tester.DebugRequest(rq, r))
		assert.Empty(t, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// error
	tester.Request("POST", "posts/error", "PAYLOAD", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusRequestEntityTooLarge, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Empty(t, r.Result().Header.Get("Content-Type"), tester.DebugRequest(rq, r))
		assert.Empty(t, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestResourceActions(t *testing.T) {
	tester.Clean()

	tester.Assign("api", &Controller{
		Model: &postModel{},
		Store: tester.Store,
		ResourceActions: M{
			"bytes": {
				Methods: []string{"POST"},
				Callback: C("bytes", All(), func(ctx *Context) error {
					assert.NotEmpty(t, ctx.Model)

					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Equal(t, []byte("PAYLOAD"), bytes)

					_, err = ctx.ResponseWriter.Write([]byte("RESPONSE"))
					assert.NoError(t, err)

					return nil
				}),
			},
			"empty": {
				Methods: []string{"POST"},
				Callback: C("empty", All(), func(ctx *Context) error {
					assert.NotEmpty(t, ctx.Model)

					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Empty(t, bytes)

					return nil
				}),
			},
			"error": {
				Methods:   []string{"POST"},
				BodyLimit: 3,
				Callback: C("error", All(), func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.Error(t, err)
					assert.Equal(t, []byte("PAY"), bytes)

					ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)

					return nil
				}),
			},
		},
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	post := tester.Save(&postModel{
		Title: "Post",
	}).(*postModel).ID()

	// get byte response
	tester.Header["Content-Type"] = "text/plain"
	tester.Header["Accept"] = "text/plain"
	tester.Request("POST", "posts/"+post.Hex()+"/bytes", "PAYLOAD", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Equal(t, "text/plain; charset=utf-8", r.Result().Header.Get("Content-Type"), tester.DebugRequest(rq, r))
		assert.Equal(t, "RESPONSE", r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get empty response
	tester.Header["Content-Type"] = ""
	tester.Header["Accept"] = ""
	tester.Request("POST", "posts/"+post.Hex()+"/empty", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Empty(t, r.Result().Header.Get("Content-Type"), tester.DebugRequest(rq, r))
		assert.Empty(t, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// get error
	tester.Request("POST", "posts/"+post.Hex()+"/error", "PAYLOAD", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusRequestEntityTooLarge, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.Empty(t, r.Result().Header.Get("Content-Type"), tester.DebugRequest(rq, r))
		assert.Empty(t, r.Body.String(), tester.DebugRequest(rq, r))
	})
}

func TestSoftDelete(t *testing.T) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model:      &postModel{},
		Store:      tester.Store,
		SoftDelete: true,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	id := tester.Save(&postModel{
		Title: "Post 1",
	}).ID().Hex()

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
						},
						"note": {
							"data": null,
							"links": {
								"self": "/posts/`+id+`/relationships/note",
								"related": "/posts/`+id+`/note"
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
					},
					"note": {
						"data": null,
						"links": {
							"self": "/posts/`+id+`/relationships/note",
							"related": "/posts/`+id+`/note"
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

	// check post
	post := tester.FindLast(&postModel{}).(*postModel)
	assert.NotNil(t, post)
	assert.NotNil(t, post.Deleted)

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

	// get missing post
	tester.Request("GET", "posts/"+id, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		assert.Equal(t, http.StatusNotFound, r.Result().StatusCode, tester.DebugRequest(rq, r))
		assert.JSONEq(t, `{
			"errors": [
				{
					"status": "404",
					"title": "Not Found",
					"detail": "resource not found"
				}
			]
		}`, r.Body.String(), tester.DebugRequest(rq, r))
	})

	// TODO: Test has one and has many relationships.
}

func BenchmarkList(b *testing.B) {
	tester.Clean()

	tester.Assign("", &Controller{
		Model: &postModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &commentModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &selectionModel{},
		Store: tester.Store,
	}, &Controller{
		Model: &noteModel{},
		Store: tester.Store,
	})

	for i := 0; i < 100; i++ {
		tester.Save(&postModel{
			Title: "Post 1",
		})
	}

	for i := 0; i < b.N; i++ {
		// get list of posts
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {})
	}
}
