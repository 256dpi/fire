<img src="https://raw.githubusercontent.com/gonfire/fire/master/doc/logo.png" alt="Logo" width="256"/>

# fire

[![Circle CI](https://img.shields.io/circleci/project/gonfire/fire.svg)](https://circleci.com/gh/gonfire/fire)
[![Coverage Status](https://coveralls.io/repos/gonfire/fire/badge.svg?branch=master&service=github)](https://coveralls.io/github/gonfire/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/gonfire/fire?status.svg)](http://godoc.org/github.com/gonfire/fire)
[![Release](https://img.shields.io/github/release/gonfire/fire.svg)](https://github.com/gonfire/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/gonfire/fire)](http://goreportcard.com/report/gonfire/fire)

**A small and opinionated framework for Go providing Ember compatible JSON APIs.**

Fire is built on top of the powerful [jsonapi](https://github.com/gonfire/jsonapi) library, uses the [mgo](https://github.com/go-mgo/mgo) MongoDB driver for persisting resources and plays well with the [gin](https://github.com/gin-gonic/gin) framework. The tight integration of these components provides a very simple API for rapidly building backend services for your Ember projects.

_The framework is still WIP and the API may be changed._

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Example](#example)
- [Installation](#installation)
- [Add-ons](#add-ons)
- [Usage](#usage)
- [Models](#models)
  - [Basics](#basics)
  - [Helpers](#helpers)
  - [Validation](#validation)
  - [Filtering & Sorting](#filtering-&-sorting)
  - [To One Relationships](#to-one-relationships)
  - [To Many Relationships](#to-many-relationships)
  - [Has Many Relationships](#has-many-relationships)
- [Controllers](#controllers)
  - [Basics](#basics-1)
  - [Callbacks](#callbacks)
  - [Built-in Callbacks](#built-in-callbacks)
- [Applications](#applications)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Example

An example application that uses the fire framework and the auth add-on to build an JSON API that is consumed by an Ember Application can be found here: <https://github.com/gonfire/example>.

## Installation

Get the package using the go tool:

```bash
$ go get github.com/gonfire/fire
```

## Add-ons

- [auth](https://github.com/gonfire/auth): OAuth2 based Authentication

## Usage

Fire infers all necessary meta information about your models from the already available `json` and `bson` struct tags. Additionally it introduces the `fire` struct tag and integrates [govalidator](https://github.com/asaskevich/govalidator) which uses the `valid` struct tag.

Such a declaration could look like the following two models for a blog system:

```go
type Post struct {
	fire.Base `bson:",inline" fire:"post:posts"`
	Title     string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody  string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments  HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
}

type Comment struct {
	fire.Base `bson:",inline" fire:"comment:comments"`
	Message   string         `json:"message" valid:"required"`
	Parent    *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID    bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}
```

Finally, an `Application` is used to mount the controller on a router and make them accessible:

```go
app := fire.NewApplication(db, "api")

app.Mount(&fire.Controller{
    Model: &Post{},
})

app.Mount(&fire.Controller{
    Model: &Comment{},
})

app.Register(router)
```

After starting the server you can inspect the created routes from the console output (simplified):

```
GET    /posts
GET    /posts/:id
GET    /posts/:id/relationships/comments
GET    /posts/:id/comments
PATCH  /posts/:id/relationships/comments
POST   /posts
DELETE /posts/:id
PATCH  /posts/:id

GET    /comments
GET    /comments/:id
GET    /comments/:id/relationships/post
GET    /comments/:id/post
PATCH  /comments/:id/relationships/post
GET    /comments/:id/relationships/parent
GET    /comments/:id/parent
PATCH  /comments/:id/relationships/parent
POST   /comments
DELETE /comments/:id
PATCH  /comments/:id
```

Fire provides various advanced features to hook into the request processing flow and add for example authentication or more complex validation of models. Please read the following API documentation carefully to get an overview of all available features.

## Models

This section describes the configuration of fire models using the right combination of struct tags.

### Basics

The embedded struct `fire.Base` has to be present in every model as it holds the document id and defines the models singular and plural name and collection via the `fire:"singular:plural[:collection]"` struct tag:

```go
type Post struct {
    fire.Base `bson:",inline" fire:"post:posts"`
    // ...
}
```

- If the collection is not explicitly set the plural name is used instead.
- The plural name of the model is also the type for to one and has many relationships.
- Fire will use the `bson` struct tag to automatically infer the database field or fallback to the lowercase version of the field name.

_Note: Ember Data requires you to use dashed names for multi-word model names like `blog-posts`._

### Helpers

The `ID` method can be used to get the document id:

```go
post.ID()
```

The `Get` and `Set` functions can be used to get and set any field on the model:

```go
title := post.Get("title")
post.Set("title", "New Title")
```

- Both methods use the field name (e.g. `TextBody`), json name (e.g. `text-body`) or bson name (e.g. `text_body`) to find the value and panic if no matching field is found.
- Calling `Set` with a different type than the field causes a panic.

The `Meta` method can be used to get the models meta structure:

```go
post.Meta().SingularName
post.Meta().PluralName
post.Meta().Collection
post.Meta().Fields
post.Meta().FieldsByTag("tag")
```

More information about the `Meta` structure can be found here: <https://godoc.org/github.com/gonfire/fire#Meta>. 

### Validation

The `Validate` method can be overridden per model to implement custom validations:

```go
func (p *Post) Validate(fresh bool) error {
    // ...

    return p.Base.Validate(fresh)
}
```

- The argument `fresh` indicates if the model has been just created.

### Filtering & Sorting

Fields can be annotated with the `fire:"filterable"` struct tag to allow filtering and with the `fire:"sortable"` struct tag to allow sorting:

```go
type Post struct {
    // ...
	Slug string `json:"slug" valid:"required" bson:"slug" fire:"filterable,sortable"`
	// ...
}
```

Filters can be activated using the `/foos?filter[field]=bar` query parameter while sorting can be specified with the `/foos?sort=field` (ascending) or `/foos?sort=-field` (descending) query parameter.

More information about filtering and sorting can be found here: <http://jsonapi.org/format/#fetching-sorting>.

### To One Relationships

Fields of the type `bson.ObjectId` or `*bson.ObjectId` can be marked as to one relationships using the `fire:"name:type"` struct tag:

```go
type Comment struct {
	// ...
	PostID bson.ObjectId `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
    // ...
}
```

- Fields of the type `*bson.ObjectId` are treated as optional relationships
- To one relationships can also have additional tags following the special relationship tag.

_Note: To one relationship fields should be excluded from the attributes object by using the `json:"-"` struct tag._

### To Many Relationships

Fields of the type `[]bson.ObjectId` can be marked as to many relationships using the `fire:"name:type"` struct tag:

```go
type Selection struct {
    // ...
	PostIDs []bson.ObjectId `json:"-" valid:"-" fire:"posts:posts"`
	// ...
}
```

- To many relationships can also have additional tags following the special relationship tag.

_Note: To many relationship fields should be excluded from the attributes object by using the `json:"-"` struct tag._

### Has Many Relationships

Fields that have a `fire.HasMany` as their type define the inverse of a to one relationship and require the `fire:"name:type:inverse"` struct tag:

```go
type Post struct {
    // ...
	Comments fire.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	// ...
}
```

Note: These fields should have the `json:"-" valid:"-" bson"-"` tag set, as they are only syntactic sugar and hold no other information.

## Controllers

This section describes the construction of fire controllers that expose the models as JSON APIs.

### Basics

Controllers are declared by creating an instance of the `Controller` type and providing a reference to the `Model`:

```go
postsController := &fire.Controller{
    Model: &Post{},
}
```

### Callbacks

Fire allows the definition of two callbacks that are called while processing the requests:

```go
posts := &fire.Controller{
    // ...
    Authorizer: func(ctx *fire.Context) error {
        // ...
    },
    Validator: func(ctx *fire.Context) error {
        // ...
    },
}
```

The `Authorizer` is run after inferring all available data from the request and is therefore perfectly suited to do a general user authentication. The `Validator` is only run before creating, updating or deleting a model and is ideal to protect resources from certain actions.

Errors returned by the callback are serialize to an JSON API compliant error object and yield an 401 (Unauthorized) status code from an Authorizer and a 400 (Bad Request) status code from a Validator.

If errors are marked as fatal a 500 (Internal Server Error) status code is returned without serializing the error to protect eventual private information:

```go
func(ctx *fire.Context) error {
    // ...
    if err != nil {
        return fire.Fatal(err)
    }
    // ...
}
```

Multiple callbacks can be combined using `fire.Combine`:

```go
fire.Combine(callback1, callback2)
```

- Execution of combined callbacks continues until an error is returned.

### Built-in Callbacks

Fire ships with several built-in callbacks that implement common concerns:

- [ProtectedAttributesValidator](https://godoc.org/github.com/gonfire/fire#ProtectedAttributesValidator)
- [DependentResourcesValidator](https://godoc.org/github.com/gonfire/fire#DependentResourcesValidator)
- [VerifyReferencesValidator](https://godoc.org/github.com/gonfire/fire#VerifyReferencesValidator)
- [MatchingReferencesValidator](https://godoc.org/github.com/gonfire/fire#MatchingReferencesValidator)

## Applications

An `Application` can be created by calling `fire.NewApplication` with a reference to a database and the full URL prefix:

```go
app := fire.NewApplication(db, "api")

app.Mount(&fire.Controller{
    Model: &Post{},
    // ...
})

app.Register(router)
````

Controllers can be mounted with `Mount` before the routes are registered using `Register` on a router.

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
