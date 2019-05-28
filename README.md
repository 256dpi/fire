<img src="http://joel-github-static.s3.amazonaws.com/gonfire/logo.png" alt="Logo" width="256"/>

# Go on Fire

[![Build Status](https://travis-ci.org/256dpi/fire.svg?branch=master)](https://travis-ci.org/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/github/256dpi/fire/badge.svg?branch=master)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**An idiomatic micro-framework for building Ember.js compatible APIs with Go.**

[Go on Fire](https://gonfire.org) is built on top of the wonderful built-in [http](https://golang.org/pkg/net/http) package, implements the [JSON API](http://jsonapi.org) specification through the dedicated [jsonapi](https://github.com/256dpi/jsonapi) library, uses the official [mongo](https://github.com/mongodb/mongo-go-driver) driver for persisting resources with [MongoDB](https://www.mongodb.com) and leverages the dedicated [oauth2](https://github.com/256dpi/oauth2) library to provide out of the box support for [OAuth2](https://oauth.net/2/) authentication using [JWT](https://jwt.io) tokens.

The deliberate and tight integration of these components provides a very simple and extensible set of abstractions for rapidly building backend services for websites that use [Ember.js](http://emberjs.com) as their frontend framework. Of course it can also be used in conjunction with any other single page application framework or as a backend for native mobile applications.

To quickly get started with building an API with Go on Fire follow the [quickstart guide](https://github.com/256dpi/fire#quickstart), read the detailed sections in this documentation and refer to the [package documentation](https://godoc.org/github.com/256dpi/fire) for more detailed information on the used types and methods.

## Example

The [example](https://github.com/256dpi/fire/tree/master/example) application implements an API will all Go on Fire features.

## Quickstart

First of all, install the package using the go tool:

```bash
$ go get -u github.com/256dpi/fire
```

Then import the `fire` package in your Go project:

```go
import "github.com/256dpi/fire"
```

### Declare Models

A basic declaration of models looks like the following example for a blog system:

```go
type Post struct {
	fire.Base  `json:"-" bson:",inline" fire:"posts"`
	Title      string        `json:"title" bson:"title"`
	TextBody   string        `json:"text-body" bson:"text_body"`
	Comments   fire.HasMany  `json:"-" bson:"-" fire:"comments:comments:post"`
}

type Comment struct {
	fire.Base  `json:"-" bson:",inline" fire:"comments"`
	Message    string         `json:"message"`
	Parent     *bson.ObjectId `json:"-" fire:"parent:comments"`
	PostID     bson.ObjectId  `json:"-" bson:"post_id" fire:"post:posts"`
}
```

Following that we need to create a store that is responsible for managing the database connection: 

```go
store := coal.MustCreateStore("mongodb://localhost/my-app")
```

### Create Controllers

Controllers make the previously declared models available from the JSON API:

```go
group := fire.NewGroup()

group.Add(&fire.Controller{
    Model: &Post{},
    Store: store,
})

group.Add(&fire.Controller{
    Model: &Comment{},
    Store: store,
})
```

### Run Application

Finally, the controller group can be served using the built-in http package:

```go
http.Handle("/api/", group.Endpoint("/api/"))

http.ListenAndServe(":4000", nil)
```

The JSON API is now available at `http://0.0.0.0:4000/api` and ready to be integrated in an Ember project.

Go on Fire provides various advanced features to hook into the request processing flow and adds for example authentication or more complex validation of models. Please read the following documentation carefully to get an overview of all available features.

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
