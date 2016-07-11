<img src="https://raw.githubusercontent.com/256dpi/fire/master/doc/logo.png" alt="Logo" width="256"/>

# fire

[![Circle CI](https://img.shields.io/circleci/project/256dpi/fire.svg)](https://circleci.com/gh/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/256dpi/fire/badge.svg?branch=master&service=github)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

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

Finally, an `Endpoint` mounts resources in a gin application and thus provides access to them:

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

### Models

This section describes the configuration of fire models using the right combination of struct tags.

#### Basics

```go
type Post struct {
    fire.Base `bson:",inline" fire:"post:posts"`
    // ...
}
```

The embedded struct `fire.Base` has to be present in every model as it holds the document id and defines the models singular and plural name via the `fire:"singular:plural"` struct tag. The plural name of the model is also the type for to one and has many relationships.

Note: Ember Data requires you to use dashed names for multi-word model names like `blog-posts`.

#### Getters

```go
post.ID()
post.Attribute("title")
comment.ReferenceID("post")
```

The `ID`, `Attribute` and `ReferenceID` functions are short-hands to access the document id, its attributes and to one relationships.

#### Validation

```go
func (p *Post) Validate(fresh bool) error {
    // ...

    return p.Base.Validate(fresh)
}
```

The `Validate` method can be overridden per model to implement custom validations.

#### Filtering & Sorting

```go
type Post struct {
    // ...
	Slug string `json:"slug" valid:"required" bson:"slug" fire:"filterable,sortable"`
	// ...
}
```

Fields can be annotated with the `fire:"filterable"` struct tag to allow filtering and with the `fire:"sortable"` struct tag to allow sorting. Filters can be activated using the `/foos?filter[field]=bar` query parameter while sorting can be specified with the `/foos?sort=field` (ascending) or `/foos?sort=-field` (descending) query parameter.

Note: Fire will use the `bson` struct tag to automatically infer the database field or fallback to the lowercase version of the field name.

#### To One Relationships

```go
type Comment struct {
	// ...
	PostID bson.ObjectId `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
    // ...
}
```

All fields of the `bson.ObjectId` type are treated as to one relationships and are required to have the `fire:"name:type"` struct tag.

Note: Fields of the type `*bson.ObjectId` are treated as optional relationships. Also the field should have the `json:"-"` struct tag to be excluded from the generated attributes object.

#### Has Many Relationships

```go
type Post struct {
    // ...
	Comments fire.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
	// ...
}
```

Fields that have a `fire.HasMany` as their type define the inverse of a to one relationship and also require the `fire:"name:type"` struct tag.

Note: These fields should have the `json:"-" valid:"-" bson"-"` tag set, as they are only syntactic sugar and hold no other information.

### Resources

This section describes the construction of fire resources that provide access to models.

#### Basics

```go
posts := &fire.Resource{
    Model:      &Post{},
    Collection: "posts"
}
```

Resources are declared by creating an instance of the `Resource` type and providing a reference to the `Model` and specifying the MongoDB `Collection`.

#### Callbacks

```go
posts := &fire.Resource{
    // ...
    Authorizer: func(ctx *fire.Context) (error, error) {
        // ...
    },
    Validator: func(ctx *fire.Context) (error, error) {
        // ...
   },
}
```

Fire allows the definition of two callbacks. The `Authorizer` is run after inferring all available data from the request and is therefore perfectly suited to do a general user authentication. The `Validator` is only run before creating, updating or deleting a model and is ideal to protect resources from certain actions.

```go
fire.Combine(callback1, callback2)
```

Multiple validators or authorizers can be combined to one callback using `fire.Combine`.

Note: Fire comes with several built-in callbacks that provide common functionalities and are well combineable with custom callbacks. Following callbacks are available:

- [DependentResourcesValidator](https://godoc.org/github.com/256dpi/fire#DependentResourcesValidator)
- [VerifyReferencesValidator](https://godoc.org/github.com/256dpi/fire#VerifyReferencesValidator)

### Endpoints

```go
endpoint := fire.NewEndpoint(db)

endpoint.AddResource(&fire.Resource{
    Model:      &Post{},
    Collection: "posts"
})

endpoint.Register(router)
````

An `Endpoint` can be creating by calling `fire.NewEndpoint` with a reference to a `mgo.Database`. Resources can be added with `AddResource` before the routes are registered on an instance that implements the `gin.IRouter` interface with `Register`.
