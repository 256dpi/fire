<img src="https://raw.githubusercontent.com/gonfire/fire/master/doc/logo.png" alt="Logo" width="256"/>

# fire

[![Build Status](https://travis-ci.org/gonfire/fire.svg?branch=master)](https://travis-ci.org/gonfire/fire)
[![Coverage Status](https://coveralls.io/repos/github/gonfire/fire/badge.svg?branch=master)](https://coveralls.io/github/gonfire/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/gonfire/fire?status.svg)](http://godoc.org/github.com/gonfire/fire)
[![Release](https://img.shields.io/github/release/gonfire/fire.svg)](https://github.com/gonfire/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/gonfire/fire)](http://goreportcard.com/report/gonfire/fire)

**A small and opinionated framework for Go providing Ember compatible JSON APIs.**

Fire is built on top of the powerful [echo](https://github.com/labstack/echo) framework, implements the JSON API specification through the streamlined [jsonapi](https://github.com/gonfire/jsonai) library and uses the very stable [mgo](https://github.com/go-mgo/mgo) MongoDB driver for persisting resources. The tight integration of these components provides a very simple API for rapidly building backend services for your Ember projects.

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
  - [Sparse Fieldsets](#sparse-fieldsets)
  - [To One Relationships](#to-one-relationships)
  - [To Many Relationships](#to-many-relationships)
  - [Has Many Relationships](#has-many-relationships)
- [Controllers](#controllers)
  - [Basics](#basics-1)
  - [Callbacks](#callbacks)
  - [Built-in Callbacks](#built-in-callbacks)
- [Controller Groups](#controller-groups)
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
	fire.Base `json:"-" bson:",inline" fire:"posts"`
	Title     string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody  string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments  HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
}

type Comment struct {
	fire.Base `json:"-" bson:",inline" fire:"comments"`
	Message   string         `json:"message" valid:"required"`
	Parent    *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID    bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
}
```

Every resource is managed by a `Controller` which provides the JSON API compliant interface:

```go
pool := NewClonePool("mongodb://localhost/my-app")

postsController := &fire.Controller{
    Model: &Post{},
    Pool: pool,
}

commentsController := &fire.Controller{
    Model: &Comment{},
    Pool: pool,
}
```

The method `NewClonePool()` returns a `Pool` that manages access to the database for the controllers. 
 
In a next step, multiple controllers are combined to a `ControllerGroup` that provides the necessary interconnection and integration with applications:

```go
group := fire.NewControllerGroup("api")

group.Add(postsController)
group.Add(commentsController)
```

To lower configuration overhead fire provides the `Application` construct which manages all the details around your API and is able to mount the `ControllerGroup` and other components of an application:

```go
app := fire.New()

app.Mount(group)

app.Start("0.0.0.0:4000")
```

Fire provides various advanced features to hook into the request processing flow and add for example authentication or more complex validation of models. Please read the following API documentation carefully to get an overview of all available features.

## Models

This section describes the configuration of models using the right combination of struct tags.

### Basics

The embedded struct `fire.Base` has to be present in every model as it holds the document id and defines the models plural and collection name via the `fire:"plural-name[:collection]"` struct tag:

```go
type Post struct {
    fire.Base `json:"-" bson:",inline" fire:"posts"`
    // ...
}
```

- If the collection is not explicitly set the plural name is used instead.
- The plural name of the model is also the type for to one, to many and has many relationships.

_Note: Ember Data requires you to use dashed names for multi-word model names like `blog-posts`._

All other fields of a structs are treated as attributes except for relationships (more on that later):

```go
type Post struct {
    // ...
    Title    string `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
    TextBody string `json:"text-body" valid:"-" bson:"text_body"`
    // ...
}
```

- Fire will use the `bson` struct tag to infer the database field or fallback to the lowercase version of the field name.
- The `json` struct tag is used for marshaling and unmarshaling the models attributes from or to a JSON API resource object. Hidden fields can be marked with the tag `json:"-"`. Fields that may only be present while creating the resource (e.g. a plain password field) can be made optional using `json:"password,omitempty"`.
- Validation is provided by [govalidator](https://github.com/asaskevich/govalidator) and uses the `valid` struct tag. All possible validations can be found [here](https://github.com/asaskevich/govalidator#validatestruct-2).

_Note: Ember Data requires you to use dashed names for multi-word attribute names like `text-body`._

### Helpers

The `ID()` method can be used to get the document id:

```go
post.ID()
```

The `Get()` and `Set()` methods can be used to get and set any field on the model:

```go
title := post.Get("title")
post.Set("title", "New Title")
```

- Both methods use the field name (e.g. `TextBody`), json name (e.g. `text-body`) or bson name (e.g. `text_body`) to find the value and panic if no matching field is found.
- Calling `Set()` with a different type than the field causes a panic.

The `Meta()` method can be used to get the models meta structure:

```go
post.Meta().Name
post.Meta().PluralName
post.Meta().Collection
post.Meta().Fields
```

More information about the `Meta` structure can be found here: <https://godoc.org/github.com/gonfire/fire#Meta>.

### Validation

The `Validate()` method can be overridden per model to implement custom validations:

```go
func (p *Post) Validate(fresh bool) error {
    // ...

    return p.Base.Validate(fresh)
}
```

- The argument `fresh` indicates if the model has been just created.
- Returned errors are serialized as a bad request error.

### Filtering & Sorting

Fields can be annotated with the `fire:"filterable"` struct tag to allow filtering and with the `fire:"sortable"` struct tag to allow sorting:

```go
type Post struct {
    // ...
	Slug string `json:"slug" valid:"required" bson:"slug" fire:"filterable,sortable"`
	// ...
}
```

Filters can be activated using the `/posts?filter[published]=true` query parameter while sorting can be specified with the `/posts?sort=created-at` (ascending) or `/posts?sort=-created-at` (descending) query parameter.

_Note: `true` and `false` are automatically converted to boolean values if the field has the `bool` type._

More information about filtering and sorting can be found here: <http://jsonapi.org/format/#fetching-sorting>.

### Sparse Fieldsets

Sparse Fieldsets are automatically supported on all responses an can be activated using the `/posts?fields[posts]=bar` query parameter.

More information about sparse fieldsets can be found here: <http://jsonapi.org/format/#fetching-sparse-fieldsets>.

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

_Note: Ember Data requires you to use dashed names for multi-word relationship names like `last-posts`._

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

_Note: Ember Data requires you to use dashed names for multi-word relationship names like `favorited-posts`._

### Has Many Relationships

Fields that have a `fire.HasMany` as their type define the inverse of a to one relationship and require the `fire:"name:type:inverse"` struct tag:

```go
type Post struct {
    // ...
	Comments fire.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments:post"`
	// ...
}
```

_Note: Ember Data requires you to use dashed names for multi-word relationship names like `authored-posts`._

Note: These fields should have the `json:"-" valid:"-" bson"-"` tag set, as they are only syntactic sugar and hold no other information.

## Controllers

This section describes the construction of controllers that expose the models as JSON APIs.

### Basics

Controllers are declared by creating a `Controller` and providing a reference to the `Model` and a pool to obtain database connections from:

```go
postsController := &fire.Controller{
    Model: &Post{},
    Pool: pool,
    // ...
}
```

### Callbacks

Fire allows the definition of two callbacks that are called while processing the requests:

```go
postsController := &fire.Controller{
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

Multiple callbacks can be combined using `fire.Combine()`:

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

## Controller Groups

Controller groups provide the necessary interconnection between controllers and the integration into existing echo applications. A `ControllerGroup` can be created by calling `fire.NewControllerGroup()` with a URL prefix while controllers are added using `Add()`:

```go
group := fire.NewControllerGroup("api")

group.Add(myController)

group.Regsiter(router)
````

Optionally, the group can be directly registered on a standard echo router using `Register()`.

## Applications

Applications provide an easy way to get started with a project. An `Application` can be created using `fire.New()` while components are mounted using `Mount()`:

```go
app := fire.New(")

app.Mount(group)

app.Start("0.0.0.0:4242")
```

An application can be started using `app.Start()` or `app.SecureStart()`.

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
