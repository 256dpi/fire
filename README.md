<img src="https://raw.githubusercontent.com/256dpi/fire/master/doc/logo.png" alt="Logo" width="256"/>

# fire

[![Circle CI](https://img.shields.io/circleci/project/256dpi/fire.svg)](https://circleci.com/gh/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/256dpi/fire/badge.svg?branch=master&service=github)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**A small and opinionated framework for Go providing Ember compatible JSON APIs.**

Fire is built on top of the amazing [api2go](https://github.com/manyminds/api2go) project, uses the [mgo](https://github.com/go-mgo/mgo) MongoDB driver for persisting resources, plays well with the [gin](https://github.com/gin-gonic/gin) framework and leverages the [fosite](https://github.com/ory-am/fosite) library to implement OAuth2 based authentication. The tight integration of these components provides a very simple API for rapidly building backend services for your Ember projects.

_The framework is still WIP and the API may be changed._

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Example](#example)
- [Installation](#installation)
- [Usage](#usage)
- [Models](#models)
  - [Basics](#basics)
  - [Helpers](#helpers)
  - [Validation](#validation)
  - [Filtering & Sorting](#filtering-&-sorting)
  - [To One Relationships](#to-one-relationships)
  - [Has Many Relationships](#has-many-relationships)
- [Resources](#resources)
  - [Basics](#basics-1)
  - [Callbacks](#callbacks)
  - [Built-in Callbacks](#built-in-callbacks)
- [Endpoints](#endpoints)
- [Authenticators](#authenticators)
  - [Scopes](#scopes)
  - [Authorization](#authorization)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Example

An example application that uses the fire framework to build an JSON API that is consumed by an Ember Application can be found here: <https://github.com/256dpi/fire-example>.

## Installation

Get the package using the go tool:

```bash
$ go get github.com/256dpi/fire
```

## Usage

Fire infers all necessary meta information about your models from the already available `json` and `bson` struct tags. Additionally it introduces the `fire` struct tag and integrates [govalidator](https://github.com/asaskevich/govalidator) which uses the `valid` struct tag.

Such a declaration could look like the following two models for a blog system:

```go
type Post struct {
	fire.Base `bson:",inline" fire:"post:posts"`
	Title     string  `json:"title" valid:"required" bson:"title" fire:"filterable,sortable"`
	TextBody  string  `json:"text-body" valid:"-" bson:"text_body"`
	Comments  HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
}

type Comment struct {
	fire.Base `bson:",inline" fire:"comment:comments"`
	Message   string         `json:"message" valid:"required"`
	Parent    *bson.ObjectId `json:"-" valid:"-" fire:"parent:comments"`
	PostID    bson.ObjectId  `json:"-" valid:"required" bson:"post_id" fire:"post:posts"`
	AuthorID  bson.ObjectId  `json:"-" valid:"required" bson:"author_id" fire:"author:users"`
}

type User struct {
	Base     `bson:",inline" fire:"user:users"`
	FullName string `json:"full_name" valid:"required"`
	Email    string `json:"email" valid:"required" fire:"identifiable"`
	Password []byte `json:"-" valid:"required"`
}
```

Finally, an `Endpoint` is used to register the resources on a router and make them accessible:

```go
endpoint := fire.NewEndpoint(db)

endpoint.AddResource(&fire.Resource{
    Model: &Post{},
})

endpoint.AddResource(&fire.Resource{
    Model: &Comment{},
})

endpoint.Register("api", router)
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
GET    /comments/:id/relationships/author
GET    /comments/:id/author
PATCH  /comments/:id/relationships/author
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
post.Meta().FieldWithTag("tag")
```

More information about the `Meta` structure can be found here: <https://godoc.org/github.com/256dpi/fire#Meta>. 

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

### Has Many Relationships

Fields that have a `fire.HasMany` as their type define the inverse of a to one relationship and require the `fire:"name:type"` struct tag:

```go
type Post struct {
    // ...
	Comments fire.HasMany `json:"-" valid:"-" bson:"-" fire:"comments:comments"`
	// ...
}
```

Note: These fields should have the `json:"-" valid:"-" bson"-"` tag set, as they are only syntactic sugar and hold no other information.

## Resources

This section describes the construction of fire resources that expose the models as JSON APIs.

### Basics

Resources are declared by creating an instance of the `Resource` type and providing a reference to the `Model`:

```go
posts := &fire.Resource{
    Model: &Post{},
}
```

### Callbacks

Fire allows the definition of two callbacks that are called while processing the requests:

```go
posts := &fire.Resource{
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

- [DependentResourcesValidator](https://godoc.org/github.com/256dpi/fire#DependentResourcesValidator)
- [VerifyReferencesValidator](https://godoc.org/github.com/256dpi/fire#VerifyReferencesValidator)
- [MatchingReferencesValidator](https://godoc.org/github.com/256dpi/fire#MatchingReferencesValidator)

## Endpoints

An `Endpoint` can be created by calling `fire.NewEndpoint` with a reference to a database:

```go
endpoint := fire.NewEndpoint(db)

endpoint.AddResource(&fire.Resource{
    Model: &Post{},
    // ...
})

endpoint.Register("api", router)
````

Resources can be added with `AddResource` before the routes are registered using `Register` on a router.

## Authenticators

An `Authenticator` provides authentication through OAuth2 and can be created using `fire.NewAuthenticator` with a reference to a database and a policy:

```go
authenticator := fire.NewAuthenticator(db, &Policy{
    Secret:           []byte("a-very-long-secret"),
    OwnerModel:       &User{},
    ClientModel:      &fire.Application{},
    AccessTokenModel: &fire.AccessToken{},
    OwnerExtractor: func(model Model) M {
        return M{
            "Secret": model.(*User).Password,
        }
    },
    EnabledGrants:    []string{PasswordGrant},
})

authenticator.Register("auth", router)
```

The owner model is required to have the tags `identifiable` and `verifiable` to allow reading the id (email or username) and secret (password hash) fields. Fire provides built-in client (application) and access token models. These models can be extended but must have exactly the same fields as the built-in ones.

After that, the necessary routes can be registered using `Register` on a router.

More information about OAuth2 can be found here: <https://www.digitalocean.com/community/tutorials/an-introduction-to-oauth-2>.

### Scopes

The default grant strategy grants all requested scopes if the client satisfies the scopes (inferred using the `grantable` tag). However, most applications want grant scopes based on client types and owner roles. A custom grant strategy can be implemented by setting a different `GrantStrategy`.

The following callback grants the `default` scope and additionally the `admin` scope if the user has the admin flag set:
 
```go
policy.GrantStrategy = func(req *GrantRequest) []string {
    list := []string{"default"}
    
    if req.Owner != nil && req.Owner.(*User).Admin {
        list = append(list, "admin")
    }

    return list
}
```

### Authorization

Later on you can use the authenticator to authorize access to your resources:

```go
posts := &fire.Resource{
    // ...
    Authorizer: authenticator.Authorizer("admin"),
}
```

The Authorizer accepts a list of scopes that must have been granted to the token.

- The authorizer will assign the AccessToken model to the context using the `fire.access_token` key.

You can also authorize plain gin handlers:
 
```go
router.GET("foo", authenticator.GinAuthorizer("admin"), func(ctx *gin.Context){
    // ...
})
```

Or use call `Authorize` while processing a request:

```go
accessToken, err := authenticator.Authorize(ctx, "admin")
```

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
