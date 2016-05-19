# fire

[![Circle CI](https://img.shields.io/circleci/project/256dpi/fire.svg)](https://circleci.com/gh/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/256dpi/fire/badge.svg?branch=master&service=github)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](http://goreportcard.com/badge/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**A small and opinionated framework for Go providing Ember Data compatible JSON APIs.**

Fire is built on top of the amazing [api2go](https://github.com/manyminds/api2go) library, uses the [mgo](https://github.com/go-mgo/mgo) MongoDB driver for persisting resources and plays well with the [gin](https://github.com/gin-gonic/gin) framework. The tight integration of these components provides a very simple API for rapidly building JSON API services for your Ember projects.

# Usage

Get the package using the go tool:

```bash
$ go get github.com/256dpi/fire
```

## Models

Describe your models using structs tags and some special fields:

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

### Base

The embedded struct `fire.Base` has to be present in every model as it holds the document `ID` and defines the models singular and plural name via the `fire:"singular:plural"` struct tag. Ember Data requires you to use dashed names for multi-word model names like `blog-posts`.

### Filter

Simple fields can be annotated with the `fire:"filter"` struct tag to allow filtering using the `/foos?filter[field]=bar` query parameter. All filterable fields need to specify the `bson:"field"` struct tag as well.

### Sorting

Simple fields can be annotated wit the `fire:"sort"` struct tag to allow sorting using the `/foos?sort=field` or `/foos?sort=-field` query parameter. All sortable fields need to specify the `bson:"field"` struct tag as well.

### To One Relationships

All fields with the `bson.ObjectId` or `*bson.ObjectId` type are treated as to one relationships and are required to have the `fire:"name:type"` struct tag. That way, the resources include the relationship links to load the relations like `/foos/1/bar`.

### Has Many Relationships

Finally, fields that have a `fire.HasMany` as their type define the inverse of a belong to relationship and also require the `fire:"name:type"` struct tag. This also generates links allows loading the related resources through `/foos/1/bars`.

## Endpoints

By declaring and endpoint, you can mount these resources in your gin application:

```go
var db *mgo.Database
var router *gin.Engine

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

