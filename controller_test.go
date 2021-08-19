package fire

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/256dpi/jsonapi/v2"
	"github.com/256dpi/serve"
	"github.com/256dpi/xo"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/256dpi/fire/coal"
	"github.com/256dpi/fire/stick"
)

func TestBasicOperations(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
					"title": "bad request",
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
					"title": "bad request",
					"detail": "resource type mismatch"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to create post with invalid id
		tester.Request("POST", "posts", `{
			"data": {
				"type": "posts",
				"id": "`+coal.New().Hex()+`"
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
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
					"title": "bad request",
					"detail": "invalid attribute",
					"source": {
						"pointer": "/data/attributes/foo"
					}
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
					"title": "bad request",
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
					"title": "bad request",
					"detail": "resource type mismatch"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to update post with invalid id
		tester.Request("PATCH", "posts/"+id, `{
			"data": {
				"type": "posts",
				"id": "`+coal.New().Hex()+`"
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			post := tester.FindLast(&postModel{})
			id = post.ID().Hex()

			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
					"detail": "invalid attribute",
					"source": {
						"pointer": "/data/attributes/foo"
					}
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
					"title": "bad request",
					"detail": "invalid resource id"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to get not existing post
		tester.Request("GET", "posts/"+coal.New().Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNotFound, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "404",
					"title": "not found",
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
					"title": "bad request",
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
	})
}

func TestHasOneRelationships(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		post := tester.Insert(&postModel{
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
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
					"detail": "invalid relationship",
					"source": {
						"pointer": "/data/relationships/foo"
					}
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
					"title": "bad request",
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
	})
}

func TestHasManyRelationships(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		existingPost := tester.Insert(&postModel{
			Title: "Post 1",
		})
		tester.Insert(&commentModel{
			Message: "Comment 1",
			Post:    existingPost.ID(),
		})

		// create new post
		post := tester.Insert(&postModel{
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
					"title": "bad request",
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
	})
}

func TestToOneRelationships(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		post1 := tester.Insert(&postModel{
			Title: "Post 1",
		}).ID().Hex()
		post2 := tester.Insert(&postModel{
			Title: "Post 2",
		}).ID().Hex()

		// create comment
		comment1 := tester.Insert(&commentModel{
			Message: "Comment 1",
			Post:    coal.MustFromHex(post1),
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
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
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
	})
}

func TestToManyRelationships(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		post1 := tester.Insert(&postModel{
			Title: "Post 1",
		}).ID().Hex()
		post2 := tester.Insert(&postModel{
			Title: "Post 2",
		}).ID().Hex()
		post3 := tester.Insert(&postModel{
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
					"title": "bad request",
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
					"title": "bad request",
					"detail": "invalid relationship id"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// unset posts relationship
		tester.Request("PATCH", "selections/"+selection+"/relationships/posts", `{
			"data": null
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
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
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
					"title": "bad request",
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
	})
}

func TestModelValidation(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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

		// create post
		post := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post.Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		})

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "error"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"detail": "validation error"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post.Hex(), `{
			"data": {
				"type": "posts",
				"id": "`+post.Hex()+`",
				"attributes": {
					"title": "error"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"detail": "validation error"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// delete
		tester.Request("DELETE", "/posts/"+post.Hex(), "{}", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.Equal(t, "", r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestSupported(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
		}, &Controller{
			Model:     &commentModel{},
			Store:     tester.Store,
			Supported: Except(List),
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
					"title": "method not allowed",
					"detail": "unsupported operation"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestFiltering(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		post1 := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID().Hex()
		post2 := tester.Insert(&postModel{
			Title:     "post-2",
			Published: false,
		}).ID().Hex()
		post3 := tester.Insert(&postModel{
			Title:     "post-3",
			Published: true,
		}).ID().Hex()

		// create selections
		selection := tester.Insert(&selectionModel{
			Name: "selection-1",
			Posts: []coal.ID{
				coal.MustFromHex(post1),
				coal.MustFromHex(post2),
				coal.MustFromHex(post3),
			},
		}).ID().Hex()
		tester.Insert(&selectionModel{
			Name: "selection-2",
		})

		// create notes
		note := tester.Insert(&noteModel{
			Title: "note-1",
			Post:  coal.MustFromHex(post1),
		}).ID().Hex()
		tester.Insert(&noteModel{
			Title: "note-2",
			Post:  coal.New(),
		})

		// test invalid filter
		tester.Request("GET", "posts?filter[foo]=bar", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors":[{
					"status": "400",
					"title": "bad request",
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
					"title": "bad request",
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
					"self": "/posts?filter[title]=post-1"
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
					"self": "/posts?filter[title]=post-2,post-3"
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
					"self": "/posts?filter[published]=true"
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
					"self": "/posts?filter[published]=false"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// test not supported relationship filter
		tester.Request("GET", "comments?filter[post]=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors":[{
					"status": "400",
					"title": "bad request",
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
					"self": "/selections/`+selection+`/posts?filter[published]=false"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// test invalid relationship filter id
		tester.Request("GET", "notes?filter[post]=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors":[{
					"status": "400",
					"title": "bad request",
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
	    			"self": "/notes?filter[post]=`+post1+`"
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
					"self": "/selections?filter[posts]=`+post1+`"
			  	}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestSorting(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		post2 := tester.Insert(&postModel{
			Title:    "post-2",
			TextBody: "body-2",
		}).ID().Hex()
		post1 := tester.Insert(&postModel{
			Title:    "post-1",
			TextBody: "body-1",
		}).ID().Hex()
		post3 := tester.Insert(&postModel{
			Title:    "post-3",
			TextBody: "body-3",
		}).ID().Hex()

		// test invalid sorter
		tester.Request("GET", "posts?sort=foo", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors":[{
					"status": "400",
					"title": "bad request",
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
					"title": "bad request",
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
					"self": "/posts?sort=title"
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
					"self": "/posts?sort=-title"
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
					"self": "/posts?sort=text-body"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create post
		post := tester.Insert(&postModel{
			Title: "Post",
		}).ID()

		// create some comments
		comment1 := tester.Insert(&commentModel{
			Message: "Comment 1",
			Post:    post,
		}).ID().Hex()
		comment2 := tester.Insert(&commentModel{
			Message: "Comment 2",
			Post:    post,
		}).ID().Hex()
		comment3 := tester.Insert(&commentModel{
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
					"self": "/posts/`+post.Hex()+`/comments?sort=message"
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
					"self": "/posts/`+post.Hex()+`/comments?sort=-message"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestProperties(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		// unknown property
		assert.PanicsWithValue(t, `fire: missing property method "Foo" for model "fire.postModel"`, func() {
			tester.Assign("", &Controller{
				Model: &postModel{},
				Properties: map[string]string{
					"Foo": "foo",
				},
			})
		})

		// invalid shape
		assert.PanicsWithValue(t, `fire: expected property method "SetTitle" for model "fire.postModel" to have no parameters and one or two return values`, func() {
			tester.Assign("", &Controller{
				Model: &postModel{},
				Properties: map[string]string{
					"SetTitle": "set-title",
				},
			})
		})

		// invalid second return value
		assert.PanicsWithValue(t, `fire: expected second return value of property method "Strings" for model "fire.postModel" to be of type error`, func() {
			tester.Assign("", &Controller{
				Model: &postModel{},
				Properties: map[string]string{
					"Strings": "strings",
				},
			})
		})

		group := tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Properties: map[string]string{
				"Virtual":      "virtual",
				"VirtualError": "virtual-error",
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

		// catch errors
		var errs []string
		group.reporter = func(err error) {
			errs = append(errs, err.Error())
		}

		// create post
		post1 := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID().Hex()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": [{
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": true,
						"text-body": "",
						"virtual": 42,
						"virtual-error": "post-1"
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
				}],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post1, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": true,
						"text-body": "",
						"virtual": 42,
						"virtual-error": "post-1"
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
				"links": {
					"self": "/posts/`+post1+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create error post
		errorPost := tester.Insert(&postModel{
			Title: "virtual-error",
		}).ID().Hex()

		// error
		tester.Request("GET", "/posts/"+errorPost, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusInternalServerError, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "500",
					"title": "internal server error"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
		assert.Equal(t, "virtual error", errs[0])

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "post-2",
					"published": true,
					"virtual": 42
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			post2 := tester.FindLast(&postModel{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post2+`",
					"attributes": {
						"title": "post-2",
						"published": true,
						"text-body": "",
						"virtual": 42,
						"virtual-error": "post-2"
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
				"links": {
					"self": "/posts/`+post2+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post1, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "Post 1",
						"published": true,
						"text-body": "",
						"virtual": 42,
						"virtual-error": "Post 1"
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
				"links": {
					"self": "/posts/`+post1+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestAuthorizers(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Authorizers: L{
				C("TestAuthorizer", All(), func(ctx *Context) error {
					return xo.SF("not authorized")
				}),
			},
		})

		// create post
		post := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusUnauthorized, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "401",
					"detail": "not authorized"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post.Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusUnauthorized, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "401",
					"detail": "not authorized"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "post-2"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusUnauthorized, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "401",
					"detail": "not authorized"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post.Hex(), `{
			"data": {
				"type": "posts",
				"id": "`+post.Hex()+`",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusUnauthorized, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "401",
					"detail": "not authorized"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// delete
		tester.Request("DELETE", "/posts/"+post.Hex(), "{}", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusUnauthorized, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "401",
					"detail": "not authorized"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestModifiers(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		var calls []Operation

		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Modifiers: L{
				C("TestModifier", All(), func(ctx *Context) error {
					ctx.Model.(*postModel).TextBody += "!!!"
					calls = append(calls, ctx.Operation)
					return nil
				}),
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

		// create post
		post1 := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
			TextBody:  "Hello",
		}).ID().Hex()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": [{
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": true,
						"text-body": "Hello"
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
				}],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post1, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": true,
						"text-body": "Hello"
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
				"links": {
					"self": "/posts/`+post1+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "post-2",
					"published": true,
					"text-body": "Hello"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			post2 := tester.FindLast(&postModel{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post2+`",
					"attributes": {
						"title": "post-2",
						"published": true,
						"text-body": "Hello!!!"
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
				"links": {
					"self": "/posts/`+post2+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post1, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "Post 1",
						"published": true,
						"text-body": "Hello!!!"
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
				"links": {
					"self": "/posts/`+post1+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// delete
		tester.Request("DELETE", "/posts/"+post1, "{}", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.Empty(t, r.Body.String(), tester.DebugRequest(rq, r))
		})

		assert.Equal(t, []Operation{Create, Update, Delete}, calls)
	})
}

func TestValidators(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Validators: L{
				C("TestValidators", All(), func(ctx *Context) error {
					return xo.SF("not valid")
				}),
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

		// create post
		post := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post.Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		})

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "post-2"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"detail": "not valid"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post.Hex(), `{
			"data": {
				"type": "posts",
				"id": "`+post.Hex()+`",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"detail": "not valid"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// delete
		tester.Request("DELETE", "/posts/"+post.Hex(), "{}", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"detail": "not valid"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestDecorators(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Decorators: L{
				C("TestDecorator", All(), func(ctx *Context) error {
					if ctx.Model != nil {
						ctx.Model.(*postModel).TextBody = "Hello World!"
					}

					for _, model := range ctx.Models {
						model.(*postModel).TextBody = "Hello World!"
					}

					return nil
				}),
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

		// create post
		post1 := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID().Hex()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": [{
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": true,
						"text-body": "Hello World!"
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
				}],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post1, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "post-1",
						"published": true,
						"text-body": "Hello World!"
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
				"links": {
					"self": "/posts/`+post1+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "post-2",
					"published": true
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			post2 := tester.FindLast(&postModel{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post2+`",
					"attributes": {
						"title": "post-2",
						"published": true,
						"text-body": "Hello World!"
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
				"links": {
					"self": "/posts/`+post2+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post1, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "Post 1",
						"published": true,
						"text-body": "Hello World!"
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
				"links": {
					"self": "/posts/`+post1+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// delete
		tester.Request("DELETE", "/posts/"+post1, "{}", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.Empty(t, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestNotifiers(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Notifiers: L{
				C("TestNotifier", All(), func(ctx *Context) error {
					if ctx.Response != nil {
						ctx.Response.Meta = jsonapi.Map{
							"Hello": "World!",
						}
					} else {
						ctx.Response = &jsonapi.Document{
							Meta: jsonapi.Map{
								"Hello": "World!",
							},
						}
					}

					return nil
				}),
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

		// create post
		post1 := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID().Hex()

		// list
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": [{
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
				}],
				"links": {
					"self": "/posts"
				},
				"meta": {
					"Hello": "World!"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// find
		tester.Request("GET", "/posts/"+post1, "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
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
					"self": "/posts/`+post1+`"
				},
				"meta": {
					"Hello": "World!"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create
		tester.Request("POST", "/posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "post-2",
					"published": true
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			post2 := tester.FindLast(&postModel{}).ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post2+`",
					"attributes": {
						"title": "post-2",
						"published": true,
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
				"links": {
					"self": "/posts/`+post2+`"
				},
				"meta": {
					"Hello": "World!"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update
		tester.Request("PATCH", "/posts/"+post1, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post1+`",
					"attributes": {
						"title": "Post 1",
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
					"self": "/posts/`+post1+`"
				},
				"meta": {
					"Hello": "World!"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// delete
		tester.Request("DELETE", "/posts/"+post1, "{}", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNoContent, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"meta": {
					"Hello": "World!"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestSparseFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Properties: map[string]string{
				"Virtual": "virtual",
			},
		}, &Controller{
			Model: &noteModel{},
			Store: tester.Store,
		})

		// create post
		post := tester.Insert(&postModel{
			Title: "Post 1",
		}).ID()

		// get posts
		tester.Request("GET", "posts/"+post.Hex()+"?fields[posts]=title,virtual,note", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "posts",
					"id": "`+post.Hex()+`",
					"attributes": {
						"title": "Post 1",
						"virtual": 42
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
					"self": "/posts/`+post.Hex()+`?fields[posts]=title,virtual,note"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create note
		note := tester.Insert(&noteModel{
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
					"self": "/posts/`+post.Hex()+`/note?fields[notes]=post"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestReadableFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model:   &postModel{},
			Store:   tester.Store,
			Filters: []string{"Title"},
			Sorters: []string{"Title"},
			Authorizers: L{
				C("TestReadableFields", All(), func(ctx *Context) error {
					assert.Equal(t, []string{"Comments", "Note", "Published", "Selections", "TextBody", "Title"}, ctx.ReadableFields)
					assert.Equal(t, []string{"Published", "TextBody", "Title"}, ctx.WritableFields)
					ctx.ReadableFields = []string{"Published"}
					return nil
				}),
			},
		}, &Controller{
			Model:   &noteModel{},
			Store:   tester.Store,
			Filters: []string{"Post"},
			Authorizers: L{
				C("TestReadableFields", All(), func(ctx *Context) error {
					assert.Equal(t, []string{"Post", "Title"}, ctx.ReadableFields)
					assert.Equal(t, []string{"Post", "Title"}, ctx.WritableFields)
					ctx.ReadableFields = []string{}
					return nil
				}),
			},
		})

		// create post
		post := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID()

		// get posts
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
					"title": "bad request",
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
					"title": "bad request",
					"detail": "relationship is not readable"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestWritableFields(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Authorizers: L{
				C("TestWritableFields", All(), func(ctx *Context) error {
					assert.Equal(t, []string{"Comments", "Note", "Published", "Selections", "TextBody", "Title"}, ctx.ReadableFields)
					assert.Equal(t, []string{"Published", "TextBody", "Title"}, ctx.WritableFields)
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
					assert.Equal(t, []string{"CreateToken", "Name", "Posts", "UpdateToken"}, ctx.ReadableFields)
					assert.Equal(t, []string{"CreateToken", "Name", "Posts", "UpdateToken"}, ctx.WritableFields)
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
					"title": "bad request",
					"detail": "field is not writable",
					"source": {
						"pointer": "Published"
					}
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create post with protected field zero value
		tester.Request("POST", "posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "Post 1",
					"published": false
				},
				"relationships": {}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.NotEmpty(t, r.Body.String(), tester.DebugRequest(rq, r))
		})

		post1 := tester.FindLast(&postModel{}).ID().Hex()

		// attempt to update post with protected field
		tester.Request("PATCH", "posts/"+post1, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`",
				"attributes": {
					"published": true
				},
				"relationships": {}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
					"detail": "field is not writable",
					"source": {
						"pointer": "Published"
					}
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update post with protected field zero value
		tester.Request("PATCH", "posts/"+post1, `{
			"data": {
				"type": "posts",
				"id": "`+post1+`",
				"attributes": {
					"published": false
				},
				"relationships": {}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.NotEmpty(t, r.Body.String(), tester.DebugRequest(rq, r))
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
							"id": "`+coal.New().Hex()+`"
						}]
					}
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
					"detail": "field is not writable",
					"source": {
						"pointer": "Posts"
					}
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// create selection with protected relationship zero value
		tester.Request("POST", "selections", `{
			"data": {
				"type": "selections",
				"attributes": {},
				"relationships": {
					"posts": {
						"data": null
					}
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.NotEmpty(t, r.Body.String(), tester.DebugRequest(rq, r))
		})

		selection1 := tester.FindLast(&selectionModel{}).ID().Hex()

		// attempt to update selection with protected relationship
		tester.Request("PATCH", "selections/"+selection1, `{
			"data": {
				"type": "selections",
				"id": "`+selection1+`",
				"relationships": {
					"posts": {
						"data": [{
							"type": "posts",
							"id": "`+coal.New().Hex()+`"
						}]
					}
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
					"detail": "field is not writable",
					"source": {
						"pointer": "Posts"
					}
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update selection with protected relationship zero value
		tester.Request("PATCH", "selections/"+selection1, `{
			"data": {
				"type": "selections",
				"id": "`+selection1+`",
				"relationships": {
					"posts": {
						"data": null
					}
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.NotEmpty(t, r.Body.String(), tester.DebugRequest(rq, r))
		})

		post2 := coal.New().Hex()

		// attempt to update posts relationship
		tester.Request("PATCH", "selections/"+selection1+"/relationships/posts", `{
			"data": [
				{
					"type": "posts",
					"id": "`+coal.New().Hex()+`"
				}
			]
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
					"detail": "relationship is not writable"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to add to posts relationship
		tester.Request("POST", "selections/"+selection1+"/relationships/posts", `{
			"data": [
				{
					"type": "posts",
					"id": "`+coal.New().Hex()+`"
				}
			]
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
					"detail": "relationship is not writable"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to remove from posts relationship
		tester.Request("DELETE", "selections/"+selection1+"/relationships/posts", `{
			"data": [
				{
					"type": "posts",
					"id": "`+post2+`"
				}
			]
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [{
					"status": "400",
					"title": "bad request",
					"detail": "relationship is not writable"
				}]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestReadableProperties(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Properties: map[string]string{
				"Virtual":      "virtual",
				"VirtualError": "virtual-error",
			},
			Authorizers: L{
				C("TestReadableProperties", All(), func(ctx *Context) error {
					assert.Equal(t, []string{"Virtual", "VirtualError"}, ctx.ReadableProperties)
					ctx.ReadableProperties = []string{"Virtual"}
					return nil
				}),
			},
		}, &Controller{
			Model: &noteModel{},
			Store: tester.Store,
			Authorizers: L{
				C("TestReadableProperties", All(), func(ctx *Context) error {
					assert.Equal(t, []string{}, ctx.ReadableProperties)
					return nil
				}),
			},
		}, &Controller{
			Model: &commentModel{},
			Store: tester.Store,
		}, &Controller{
			Model: &selectionModel{},
			Store: tester.Store,
		})

		// create post
		post := tester.Insert(&postModel{
			Title:     "post-1",
			Published: true,
		}).ID().Hex()

		// get posts
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post+`",
						"attributes": {
							"title": "post-1",
							"published": true,
							"text-body": "",
							"virtual": 42
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
					}
				],
				"links": {
					"self": "/posts"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestRelationshipFilters(t *testing.T) {
	// TODO: Support to one relationships?
	// TODO: Support to many relationships?

	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Authorizers: L{
				C("TestRelationshipFilters", All(), func(ctx *Context) error {
					ctx.RelationshipFilters = map[string][]bson.M{
						"Comments": {
							{
								"Message": "bar",
							},
						},
						"Selections": {
							{
								"Name": "bar",
							},
						},
						"Note": {
							{
								"Title": "bar",
							},
						},
					}
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
				C("TestRelationshipFilters", All(), func(ctx *Context) error {
					ctx.RelationshipFilters = map[string][]bson.M{
						"Posts": {
							{
								"Title": "bar",
							},
						},
					}
					return nil
				}),
			},
		}, &Controller{
			Model: &noteModel{},
			Store: tester.Store,
			Authorizers: L{
				C("TestRelationshipFilters", All(), func(ctx *Context) error {
					ctx.RelationshipFilters = map[string][]bson.M{
						"Post": {
							{
								"Title": "x",
							},
						},
					}
					return nil
				}),
			},
		})

		// create post
		post := tester.Insert(&postModel{
			Title: "post",
		}).ID().Hex()

		// create comment
		comment1 := coal.New().Hex()
		tester.Insert(&commentModel{
			Base:    coal.B(coal.MustFromHex(comment1)),
			Message: "foo",
			Parent:  coal.P(coal.MustFromHex(comment1)),
			Post:    coal.MustFromHex(post),
		})
		comment2 := coal.New().Hex()
		tester.Insert(&commentModel{
			Base:    coal.B(coal.MustFromHex(comment2)),
			Message: "bar",
			Parent:  coal.P(coal.MustFromHex(comment2)),
			Post:    coal.MustFromHex(post),
		})

		// create selection
		tester.Insert(&selectionModel{
			Name: "foo",
			Posts: []coal.ID{
				coal.MustFromHex(post),
			},
		})
		selection2 := tester.Insert(&selectionModel{
			Name: "bar",
			Posts: []coal.ID{
				coal.MustFromHex(post),
			},
		}).ID().Hex()

		// create notes
		tester.Insert(&noteModel{
			Title: "foo",
			Post:  coal.MustFromHex(post),
		})
		note2 := tester.Insert(&noteModel{
			Title: "bar",
			Post:  coal.MustFromHex(post),
		}).ID().Hex()

		// get posts
		tester.Request("GET", "posts", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": [
					{
						"type": "posts",
						"id": "`+post+`",
						"attributes": {
							"title": "post",
							"published": false,
							"text-body": ""
						},
						"relationships": {
							"comments": {
								"data": [
									{
										"id": "`+comment2+`",
										"type": "comments"
									}
								],
								"links": {
									"self": "/posts/`+post+`/relationships/comments",
									"related": "/posts/`+post+`/comments"
								}
							},
							"selections": {
								"data": [
									{
										"id": "`+selection2+`",
										"type": "selections"
									}
								],
								"links": {
									"self": "/posts/`+post+`/relationships/selections",
									"related": "/posts/`+post+`/selections"
								}
							},
							"note": {
								"data": {
									"id": "`+note2+`",
									"type": "notes"
								},
								"links": {
									"self": "/posts/`+post+`/relationships/note",
									"related": "/posts/`+post+`/note"
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

		// // get comments
		// tester.Request("GET", "comments", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		// 	assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		// 	assert.JSONEq(t, `{
		// 		"data": [
		// 			{
		// 				"type": "comments",
		// 				"id": "`+comment+`",
		// 				"attributes": {
		// 					"message": "comment"
		// 				},
		// 				"relationships": {
		// 					"parent": {
		// 						"data": [],
		// 						"links": {
		// 							"self": "/comments/`+comment+`/relationships/parent",
		// 							"related": "/comments/`+comment+`/parent"
		// 						}
		// 					},
		// 					"post": {
		// 						"data": null,
		// 						"links": {
		// 							"self": "/comments/`+comment+`/relationships/post",
		// 							"related": "/comments/`+comment+`/post"
		// 						}
		// 					}
		// 				}
		// 			}
		// 		],
		// 		"links": {
		// 			"self": "/comments"
		// 		}
		// 	}`, r.Body.String(), tester.DebugRequest(rq, r))
		// })

		// // get selections
		// tester.Request("GET", "selections", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		// 	assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		// 	assert.JSONEq(t, `{
		// 		"data": [
		// 			{
		// 				"type": "selections",
		// 				"id": "`+selection+`",
		// 				"attributes": {
		// 					"message": "comment"
		// 				},
		// 				"relationships": {
		// 					"posts": {
		// 						"data": [],
		// 						"links": {
		// 							"self": "/selections/`+selection+`/relationships/posts",
		// 							"related": "/selections/`+selection+`/posts"
		// 						}
		// 					}
		// 				}
		// 			}
		// 		],
		// 		"links": {
		// 			"self": "/selections"
		// 		}
		// 	}`, r.Body.String(), tester.DebugRequest(rq, r))
		// })

		// // get notes
		// tester.Request("GET", "notes", "", func(r *httptest.ResponseRecorder, rq *http.Request) {
		// 	assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
		// 	assert.JSONEq(t, `{
		// 		"data": [
		// 			{
		// 				"type": "notes",
		// 				"id": "`+note+`",
		// 				"attributes": {
		// 					"message": "note"
		// 				},
		// 				"relationships": {
		// 					"post": {
		// 						"data": null,
		// 						"links": {
		// 							"self": "/notes/`+note+`/relationships/post",
		// 							"related": "/notes/`+note+`/post"
		// 						}
		// 					}
		// 				}
		// 			}
		// 		],
		// 		"links": {
		// 			"self": "/notes"
		// 		}
		// 	}`, r.Body.String(), tester.DebugRequest(rq, r))
		// })
	})
}

func TestDatabaseErrors(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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

		// missing resource
		tester.Request("GET", "posts/"+coal.New().Hex(), "", func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusNotFound, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "404",
						"title": "not found",
						"detail": "resource not found"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// add unique index
		catalog := coal.NewCatalog(&postModel{})
		catalog.AddIndex(&postModel{}, true, 0, "Title")
		err := catalog.EnsureIndexes(tester.Store)
		assert.NoError(t, err)

		// first post
		tester.Request("POST", "posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
		})

		// second post
		tester.Request("POST", "posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "Post 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "400",
						"title": "bad request",
						"detail": "document is not unique"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// remove index
		_, err = tester.Store.C(&postModel{}).Native().Indexes().DropAll(nil)
		assert.NoError(t, err)
	})
}

func TestTolerateViolations(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Authorizers: L{
				C("TestWritableFields", All(), func(ctx *Context) error {
					ctx.WritableFields = []string{"Title"}
					return nil
				}),
			},
			TolerateViolations: []string{"Published"},
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
			TolerateViolations: []string{"Posts"},
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
							"id": "`+coal.New().Hex()+`"
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
	})
}

func TestPagination(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
		var ids []coal.ID

		// create some posts
		for i := 0; i < 10; i++ {
			ids = append(ids, tester.Insert(&postModel{
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
		selection := tester.Insert(&selectionModel{
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
		post := tester.Insert(&postModel{
			Title: "Post",
		}).ID()

		// create some comments
		for i := 0; i < 10; i++ {
			tester.Insert(&commentModel{
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
	})
}

func TestListLimit(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
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
			tester.Insert(&postModel{
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
	})
}

func TestCollectionActions(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("api", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			CollectionActions: M{
				"bytes": A("bytes", []string{"POST"}, 0, func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Equal(t, []byte("PAYLOAD"), bytes)

					_, err = ctx.ResponseWriter.Write([]byte("RESPONSE"))
					assert.NoError(t, err)

					return nil
				}),
				"empty": A("empty", []string{"POST"}, 0, func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Empty(t, bytes)

					return nil
				}),
				"error": A("error", []string{"POST"}, 3, func(ctx *Context) error {
					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.Error(t, err)
					assert.Equal(t, []byte{}, bytes)

					ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)

					return nil
				}),
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
	})
}

func TestResourceActions(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		tester.Assign("api", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			ResourceActions: M{
				"bytes": A("bytes", []string{"POST"}, 0, func(ctx *Context) error {
					assert.NotEmpty(t, ctx.Model)

					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Equal(t, []byte("PAYLOAD"), bytes)

					_, err = ctx.ResponseWriter.Write([]byte("RESPONSE"))
					assert.NoError(t, err)

					return nil
				}),
				"empty": A("empty", []string{"POST"}, 0, func(ctx *Context) error {
					assert.NotEmpty(t, ctx.Model)

					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.NoError(t, err)
					assert.Empty(t, bytes)

					return nil
				}),
				"error": A("error", []string{"POST"}, 3, func(ctx *Context) error {
					assert.NotEmpty(t, ctx.Model)

					bytes, err := ioutil.ReadAll(ctx.HTTPRequest.Body)
					assert.Error(t, err)
					assert.Equal(t, []byte{}, bytes)
					assert.Equal(t, serve.ErrBodyLimitExceeded, err)

					ctx.ResponseWriter.WriteHeader(http.StatusRequestEntityTooLarge)

					return nil
				}),
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

		post := tester.Insert(&postModel{
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
	})
}

func TestSoftDelete(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		// missing field on model
		assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "fire-soft-delete" on "fire.missingSoftDeleteField"`, func() {
			type missingSoftDeleteField struct {
				coal.Base `json:"-" bson:",inline" coal:"models"`
				stick.NoValidation
			}

			tester.Assign("", &Controller{
				Model:      &missingSoftDeleteField{},
				SoftDelete: true,
			})
		})

		// invalid field type
		assert.PanicsWithValue(t, `fire: soft delete field "Foo" for model "fire.invalidSoftDeleteFieldType" is not of type "*time.Time"`, func() {
			type invalidSoftDeleteFieldType struct {
				coal.Base `json:"-" bson:",inline" coal:"models"`
				Foo       int `coal:"fire-soft-delete"`
				stick.NoValidation
			}

			tester.Assign("", &Controller{
				Model:      &invalidSoftDeleteFieldType{},
				SoftDelete: true,
			})
		})

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

		id := tester.Insert(&postModel{
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
						"title": "not found",
						"detail": "resource not found"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// TODO: Test has one and has many relationships.
	})
}

func TestIdempotentCreate(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		// missing field on model
		assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "fire-idempotent-create" on "fire.missingIdempotentCreateField"`, func() {
			type missingIdempotentCreateField struct {
				coal.Base `json:"-" bson:",inline" coal:"models"`
				stick.NoValidation
			}

			tester.Assign("", &Controller{
				Model:            &missingIdempotentCreateField{},
				IdempotentCreate: true,
			})
		})

		// invalid field type
		assert.PanicsWithValue(t, `fire: idempotent create field "Foo" for model "fire.invalidIdempotentCreateFieldType" is not of type "string"`, func() {
			type invalidIdempotentCreateFieldType struct {
				coal.Base `json:"-" bson:",inline" coal:"models"`
				Foo       int `coal:"fire-idempotent-create"`
				stick.NoValidation
			}

			tester.Assign("", &Controller{
				Model:            &invalidIdempotentCreateFieldType{},
				IdempotentCreate: true,
			})
		})

		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
		}, &Controller{
			Model: &commentModel{},
			Store: tester.Store,
		}, &Controller{
			Model:            &selectionModel{},
			Store:            tester.Store,
			IdempotentCreate: true,
		}, &Controller{
			Model: &noteModel{},
			Store: tester.Store,
		})

		// missing create token
		tester.Request("POST", "selections", `{
			"data": {
				"type": "selections",
				"attributes": {
					"name": "test"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "400",
						"title": "bad request",
						"detail": "missing idempotent create token"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		var id string

		// create selection
		tester.Request("POST", "selections", `{
			"data": {
				"type": "selections",
				"attributes": {
					"name": "Selection 1",
					"create-token": "foo123"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			selection := tester.FindLast(&selectionModel{})
			id = selection.ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "selections",
					"id": "`+id+`",
					"attributes": {
						"name": "Selection 1",
						"create-token": "foo123"
					},
					"relationships": {
						"posts": {
							"data": [],
							"links": {
								"self": "/selections/`+id+`/relationships/posts",
								"related": "/selections/`+id+`/posts"
							}
						}
					}
				},
				"links": {
					"self": "/selections/`+id+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to create duplicate
		tester.Request("POST", "selections", `{
			"data": {
				"type": "selections",
				"attributes": {
					"name": "Selection 1",
					"create-token": "foo123"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusConflict, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "409",
						"title": "conflict",
						"detail": "existing document with same idempotent create token"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// attempt to change create token
		tester.Request("PATCH", "selections/"+id, `{
			"data": {
				"type": "selections",
				"id": "`+id+`",
				"attributes": {
					"create-token": "bar456"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "400",
						"title": "bad request",
						"detail": "idempotent create token cannot be changed"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestConsistentUpdate(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		// missing field on model
		assert.PanicsWithValue(t, `coal: no or multiple fields flagged as "fire-consistent-update" on "fire.missingConsistentUpdateField"`, func() {
			type missingConsistentUpdateField struct {
				coal.Base `json:"-" bson:",inline" coal:"models"`
				stick.NoValidation
			}

			tester.Assign("", &Controller{
				Model:            &missingConsistentUpdateField{},
				ConsistentUpdate: true,
			})
		})

		// invalid field type
		assert.PanicsWithValue(t, `fire: consistent update field "Foo" for model "fire.invalidConsistentUpdateFieldType" is not of type "string"`, func() {
			type invalidConsistentUpdateFieldType struct {
				coal.Base `json:"-" bson:",inline" coal:"models"`
				Foo       int `coal:"fire-consistent-update"`
				stick.NoValidation
			}

			tester.Assign("", &Controller{
				Model:            &invalidConsistentUpdateFieldType{},
				ConsistentUpdate: true,
			})
		})

		tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
		}, &Controller{
			Model: &commentModel{},
			Store: tester.Store,
		}, &Controller{
			Model:            &selectionModel{},
			Store:            tester.Store,
			ConsistentUpdate: true,
		}, &Controller{
			Model: &noteModel{},
			Store: tester.Store,
		})

		var id string
		var selection *selectionModel

		// create selection
		tester.Request("POST", "selections", `{
			"data": {
				"type": "selections",
				"attributes": {
					"name": "Selection 1"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			selection = tester.FindLast(&selectionModel{}).(*selectionModel)
			id = selection.ID().Hex()

			assert.Equal(t, http.StatusCreated, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "selections",
					"id": "`+id+`",
					"attributes": {
						"name": "Selection 1",
						"update-token": "`+selection.UpdateToken+`"
					},
					"relationships": {
						"posts": {
							"data": [],
							"links": {
								"self": "/selections/`+id+`/relationships/posts",
								"related": "/selections/`+id+`/posts"
							}
						}
					}
				},
				"links": {
					"self": "/selections/`+id+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// missing update token
		tester.Request("PATCH", "selections/"+id, `{
			"data": {
				"type": "selections",
				"id": "`+id+`",
				"attributes": {}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "400",
						"title": "bad request",
						"detail": "invalid consistent update token"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// invalid update token
		tester.Request("PATCH", "selections/"+id, `{
			"data": {
				"type": "selections",
				"id": "`+id+`",
				"attributes": {
					"update-token": "bar123"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusBadRequest, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "400",
						"title": "bad request",
						"detail": "invalid consistent update token"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		// update selection
		tester.Request("PATCH", "selections/"+id, `{
			"data": {
				"type": "selections",
				"id": "`+id+`",
				"attributes": {
					"update-token": "`+selection.UpdateToken+`"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			selection = tester.FindLast(&selectionModel{}).(*selectionModel)

			assert.Equal(t, http.StatusOK, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"data": {
					"type": "selections",
					"id": "`+id+`",
					"attributes": {
						"name": "Selection 1",
						"update-token": "`+selection.UpdateToken+`"
					},
					"relationships": {
						"posts": {
							"data": [],
							"links": {
								"self": "/selections/`+id+`/relationships/posts",
								"related": "/selections/`+id+`/posts"
							}
						}
					}
				},
				"links": {
					"self": "/selections/`+id+`"
				}
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})
	})
}

func TestTransactions(t *testing.T) {
	withTester(t, func(t *testing.T, tester *Tester) {
		group := tester.Assign("", &Controller{
			Model: &postModel{},
			Store: tester.Store,
			Notifiers: L{
				C("foo", All(), func(ctx *Context) error {
					if ctx.Model.(*postModel).Title == "FAIL" {
						return xo.F("foo")
					}
					return nil
				}),
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

		var errs []string
		group.reporter = func(err error) {
			errs = append(errs, err.Error())
		}

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

		// create post error
		tester.Request("POST", "posts", `{
			"data": {
				"type": "posts",
				"attributes": {
					"title": "FAIL"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			post := tester.FindLast(&postModel{})
			id = post.ID().Hex()

			assert.Equal(t, http.StatusInternalServerError, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "500",
						"title": "internal server error"
					}
				]
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

		// update post
		tester.Request("PATCH", "posts/"+id, `{
			"data": {
				"type": "posts",
				"id": "`+id+`",
				"attributes": {
					"title": "FAIL"
				}
			}
		}`, func(r *httptest.ResponseRecorder, rq *http.Request) {
			assert.Equal(t, http.StatusInternalServerError, r.Result().StatusCode, tester.DebugRequest(rq, r))
			assert.JSONEq(t, `{
				"errors": [
					{
						"status": "500",
						"title": "internal server error"
					}
				]
			}`, r.Body.String(), tester.DebugRequest(rq, r))
		})

		assert.Equal(t, 1, tester.Count(&postModel{}))
		assert.Equal(t, "Post 1", stick.MustGet(tester.Fetch(&postModel{}, coal.MustFromHex(id)), "Title"))

		assert.Equal(t, []string{"foo", "foo"}, errs)
	})
}
