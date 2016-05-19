<img src="https://raw.githubusercontent.com/256dpi/fire/master/doc/logo.png" alt="Logo" width="256"/>

# fire

[![Circle CI](https://img.shields.io/circleci/project/256dpi/fire.svg)](https://circleci.com/gh/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/256dpi/fire/badge.svg?branch=master&service=github)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](http://goreportcard.com/badge/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**A small and opinionated framework for Go providing Ember Data compatible JSON APIs.**

Fire is built on top of the amazing [api2go](https://github.com/manyminds/api2go) library, uses the [mgo](https://github.com/go-mgo/mgo) MongoDB driver for persisting resources and plays well with the [gin](https://github.com/gin-gonic/gin) framework. The tight integration of these components provides a very simple API for rapidly building JSON API services for your Ember projects.

# Installation

Get the package using the go tool:

```bash
$ go get github.com/256dpi/fire
```

# Usage

Fire infers all necessary meta information about your models from the already available `json` and `bson` struct tags. Additionally it introduces the `fire` struct tag and integrates [govalidator](https://github.com/asaskevich/govalidator) which uses the `valid` struct tag.

Such a declaration could look like the following two models for a blog system:

```go
type Post struct {
	fire.Base `bson:",inline" fire:"post:posts"`
	Slug      string         `json:"slug" valid:"required" bson:"slug" fire:"filter,sort"`
	Title     string         `json:"title" valid:"required"`
	TextBody  string         `json:"text-body" valid:"-" bson:"text_body"`
	Comments  fire.HasMany   `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	fire.Base `bson:",inline" fire:"comment:comments"`
	Message   string         `json:"message" valid:"required"`
	PostID    bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
	AuthorID  *bson.ObjectId `json:"-" valid:"-" bson:"author_id" fire:"user:users"`
}
```

Finally, an `Endpoint` manages and provides access to these resources:

```go
var db *mgo.Database // a reference to a database from a mgo.Session
var router gin.IRouter // a reference to a gin router compatible instance

endpoint := fire.NewEndpoint(db)

endpoint.AddResource(&fire.Resource{
    Model:      &Post{},
    Collection: "posts"
})

endpoint.AddResource(&fire.Resource{
    Model:       &Comment{},
    Collection: "comments"
})

endpoint.Register(router)
```

After starting the gin server you can inspect the created routes from the console output (simplified):

```
GET     /posts
GET     /posts/:id
GET     /posts/:id/relationships/next-post
GET     /posts/:id/next-post
PATCH   /posts/:id/relationships/next-post
GET     /posts/:id/relationships/comments
GET     /posts/:id/comments
PATCH   /posts/:id/relationships/comments
POST    /posts
DELETE  /posts/:id
PATCH   /posts/:id
GET     /comments
GET     /comments/:id
GET     /comments/:id/relationships/post
GET     /comments/:id/post
PATCH   /comments/:id/relationships/post
POST    /comments
DELETE  /comments/:id
PATCH   /comments/:id
```

Fire provides various advanced features to hook into the request processing flow and add for example authentication or more complex validation of models. Please read the following API documentation carefully to get an overview of all available features.

## API

### Model

#### Base

```go
type Post struct {
    fire.Base `bson:",inline" fire:"post:posts"`
    // ...
}
```

The embedded struct `fire.Base` has to be present in every model as it holds the document id and defines the models singular and plural name via the `fire:"singular:plural"` struct tag.

Note: Ember Data requires you to use dashed names for multi-word model names like `blog-posts`.

#### Sorting & Filtering

```go
type Post struct {
    // ...
	Slug string `json:"slug" valid:"required" bson:"slug" fire:"filterable,sortable"`
	// ...
}
```

Simple fields can be annotated with the `fire:"filter"` struct tag to allow filtering and with the `fire:"sort"` struct tag to allow sorting. Filters can be activated using the `/foos?filter[field]=bar` query parameter while sorting can be specified with the `/foos?sort=field` (ascending) or `/foos?sort=-field` (descending) query parameter.

Note: Fire will use the `bson` struct tag to automatically infer the database field or fallback to the lowercase version of the field name.

#### To One Relationships

All fields with the `bson.ObjectId` or `*bson.ObjectId` type are treated as to one relationships and are required to have the `fire:"name:type"` struct tag. That way, the resources include the relationship links to load the relations like `/foos/1/bar`.

#### Has Many Relationships

Finally, fields that have a `fire.HasMany` as their type define the inverse of a belong to relationship and also require the `fire:"name:type"` struct tag. This also generates links allows loading the related resources through `/foos/1/bars`.

### Endpoint

#### Callbacks
