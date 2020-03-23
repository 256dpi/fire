<img src="http://joel-github-static.s3.amazonaws.com/gonfire/logo.png" alt="Logo" width="256"/>

# Go on Fire

[![Build Status](https://travis-ci.org/256dpi/fire.svg?branch=master)](https://travis-ci.org/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/github/256dpi/fire/badge.svg?branch=master)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**An idiomatic micro-framework for building Ember.js compatible APIs with Go.**

- [Introduction](#introduction)
- [Features](#features)
- [Example](#example)
- [Installation](#installation)
- [Models](#models)
- [Controllers](#controllers)
- [Authentication](#authentication)
- [Authorization](#authorization)

<!-- BEGIN DOCS -->

## Introduction

[Go on Fire](https://gonfire.org) is built on top of the wonderful built-in [http](https://golang.org/pkg/net/http) package, implements the [JSON API](http://jsonapi.org) specification through the dedicated [jsonapi](https://github.com/256dpi/jsonapi) library, uses the official [mongo](https://github.com/mongodb/mongo-go-driver) driver for persisting resources with [MongoDB](https://www.mongodb.com) and leverages the dedicated [oauth2](https://github.com/256dpi/oauth2) library to provide out of the box support for [OAuth2](https://oauth.net/2/) authentication using [JWT](https://jwt.io) tokens. Additionally, it provides packages for request authorization, asynchronous job processing and WebSocket/SSE based event sourcing.  

The deliberate and tight integration of these components provides a very simple and extensible set of abstractions for rapidly building backend services for websites that use [Ember.js](http://emberjs.com) as their frontend framework. Of course it can also be used in conjunction with any other single page application framework or as a backend for native mobile applications.

To quickly get started with building an API with Go on Fire read the following sections in this documentation and refer to the [package documentation](https://godoc.org/github.com/256dpi/fire) for more detailed information on the used types and methods.

## Features

Go on Fire ships with builtin support for various features to also provide a complete toolkit for advanced projects:

- Declarative definition of models and resource controllers.
- Custom group, collection and resource actions.
- Builtin validators incl. automatic relationship validation.
- Callback based plugin system for easy extendability.
- Integrated asynchronous and distributed job processing system.
- Event sourcing via WebSockets and SSE.
- Declarative authentication and authorization framework.
- Integrated OAuth2 authenticator and authorizer.
- Support for tracing via [opentracing](https://opentracing.io).

## Example

The [example](https://github.com/256dpi/fire/tree/master/example) application implements an API that uses most Go on Fire features.

## Installation

To get started, install the package using the go tool:

```bash
$ go get -u github.com/256dpi/fire
```

## Models

Go on Fire implements a small introspection library that is able to infer all necessary meta information about your models from the already available `json` and `bson` struct tags. Additionally it introduces the `coal` struct tag that is used to declare to-one, to-many and has-many relationships.

### Basics

The [`Base`](https://godoc.org/github.com/256dpi/fire/coal#Base) struct has to be embedded in every Go on Fire model as it holds the document id and defines the models plural name and collection via the `coal:"plural-name[:collection]"` struct tag:

```go
type Post struct {
    coal.Base `json:"-" bson:",inline" coal:"posts"`
    // ...
}
```

- If the collection is not explicitly set the plural name is used instead.
- The plural name of the model is also the type for to-one, to-many and has-many relationships.

Note: Ember Data requires you to use dashed names for multi-word model names like `blog-posts`.

All other fields of a struct are treated as attributes except for relationships (more on that later):

```go
type Post struct {
    // ...
    Title    string `json:"title" bson:"title"`
    TextBody string `json:"text-body" bson:"text_body"`
    // ...
}
```

- The `bson` struct tag is used to infer the database field or fallback to the lowercase version of the field name.
- The `json` struct tag is used for marshaling and unmarshaling the models attributes from or to a JSON API resource object. Hidden fields can be marked with the tag `json:"-"`.
- Fields that may only be present while creating the resource (e.g. a plain password field) can be made optional and temporary using `json:"password,omitempty" bson:"-"`.
- The `coal` tag may be used on fields to tag them with custom and builtin tags.

Note: Ember Data requires you to use dashed names for multi-word attribute names like `text-body`.

### Helpers

The [`ID`](https://godoc.org/github.com/256dpi/fire/coal#Base.ID) method can be used to get the document id:

```go
post.ID()
```

The [`MustGet`](https://godoc.org/github.com/256dpi/fire/coal#Base.MustGet) and [`MustSet`](https://godoc.org/github.com/256dpi/fire/coal#Base.MustSet) methods can be used to get and set any field on the model:

```go
title := post.MustGet("Title")
post.MustSet("Title", "New Title")
```

- Both methods use the field name e.g. `TextBody` to find the value and panic if no matching field is found.
- Calling [`MustSet`](https://godoc.org/github.com/256dpi/fire#Base.MustSet) with a different type than the field causes a panic.

### Meta

All parsed information from the model struct and its tags is saved to the [`Meta`](https://godoc.org/github.com/256dpi/fire/coal#Meta) struct that can be accessed using the [`Meta`](https://godoc.org/github.com/256dpi/fire/coal#Base.Meta) method:

```go
post.Meta().Name
post.Meta().PluralName
post.Meta().Collection
post.Meta().Fields
post.Meta().OrderedFields
post.Meta().DatabaseFields
post.Meta().Attributes
post.Meta().Relationships
post.Meta().FlaggedFields
```

- The `Meta` struct is read-only and should not be modified.

### To-One Relationships

Fields of the type `coal.ID` can be marked as to-one relationships using the `coal:"name:type"` struct tag:

```go
type Comment struct {
	// ...
	Post coal.ID `json:"-" bson:"post_id" coal:"post:posts"`
    // ...
}
```

- Fields of the type `*coal.ID` are treated as optional relationships.

Note: To-one relationship fields should be excluded from the attributes object by using the `json:"-"` struct tag.

Note: Ember Data requires you to use dashed names for multi-word relationship names like `last-posts`.

### To-Many Relationships

Fields of the type `[]coal.ID` can be marked as to-many relationships using the `coal:"name:type"` struct tag:

```go
type Selection struct {
    // ...
	Posts []coal.ID `json:"-" bson:"post_ids" coal:"posts:posts"`
	// ...
}
```

Note: To-many relationship fields should be excluded from the attributes object by using the `json:"-"` struct tag.

Note: Ember Data requires you to use dashed names for multi-word relationship names like `favorited-posts`.

### Has-Many Relationships

Fields that have a `coal.HasMany` as their type define the inverse of a to-one relationship and require the `coal:"name:type:inverse"` struct tag:

```go
type Post struct {
    // ...
	Comments coal.HasMany `json:"-" bson:"-" coal:"comments:comments:post"`
	// ...
}
```

Note: Ember Data requires you to use dashed names for multi-word relationship names like `authored-posts`.

Note: These fields should have the `json:"-" bson"-"` tag set, as they are only syntactic sugar and hold no other information.

### Stores

Access to the database is managed using the [`Store`](https://godoc.org/github.com/256dpi/fire/coal#Store) struct: 

```go
store := coal.MustCreateStore("mongodb://localhost/my-app")
```

The [`C`](https://godoc.org/github.com/256dpi/fire/coal#Store.C) method can be used to easily get the collection for a model:

```go
coll := store.C(&Post{})
```

The store does not provide other typical ORM methods that wrap the underlying driver, instead custom code should use the driver directly to get access to all offered features.

### Advanced Features

The `coal` package offers the following advanced features:

- [`Stream`](https://godoc.org/github.com/256dpi/fire/coal#Stream) uses MongoDB change streams to provide an event source of created, updated and deleted models.
- [`Reconcile`](https://godoc.org/github.com/256dpi/fire/coal#Reconcile) uses streams to provide an simple API to synchronize a collection of models.
- [`Catalog`](https://godoc.org/github.com/256dpi/fire/coal#Catalog) serves as a registry for models and indexes and allows the rendering of and ERD using `graphviz`.
- Various helpers to DRY up the code.

## Controllers

Go on Fire implements the JSON API specification and provides the management of the previously declared models via a set of controllers that are combined to a group which provides the necessary interconnection between resources.

Controllers are declared by creating a [`Controller`](https://godoc.org/github.com/256dpi/fire#Controller) and providing a reference to the model and store:

```go
postsController := &fire.Controller{
    Model: &Post{},
    Store: store,
    // ...
}
```

### Groups

Controller groups provide the necessary interconnection and integration between controllers as well as the main endpoint for incoming requests. A [`Group`](https://godoc.org/github.com/256dpi/fire#Group) can be created by calling [`NewGroup`](https://godoc.org/github.com/256dpi/fire#NewGroup) while controllers are added using [`Add`](https://godoc.org/github.com/256dpi/fire#Group.Add):

```go
group := fire.NewGroup()
group.Add(postsController)
group.Add(commentsController)
````

The controller group can be served using the built-in http package:

```go
http.Handle("/api/", group.Endpoint("/api/"))
http.ListenAndServe(":4000", nil)
```

The JSON API is now available at `http://0.0.0.0:4000/api`.

### Filtering & Sorting

To enable the built-in support for filtering and sorting via the JSON API specification you need to specify the allowed fields for each feature:

```go
postsController := &fire.Controller{
    // ...
    Filters: []string{"Title", "Published"},
    Sorters: []string{"Title"},
    // ...
}
```

Filters can be activated using the `/posts?filter[published]=true` query parameter while the sorting can be specified with the `/posts?sort=created-at` (ascending) or `/posts?sort=-created-at` (descending) query parameter.

Note: `true` and `false` are automatically converted to boolean values if the field has the `bool` type.

More information about filtering and sorting can be found in the [JSON API Spec](http://jsonapi.org/format/#fetching-sorting).

### Sparse Fieldsets

Sparse Fieldsets are automatically supported on all responses an can be activated using the `/posts?fields[posts]=bar` query parameter.

More information about sparse fieldsets can be found in the [JSON API Spec](http://jsonapi.org/format/#fetching-sparse-fieldsets).

### Callbacks

Controllers support the definition of multiple callbacks that are called while processing the requests:

```go
postsController := &fire.Controller{
    // ...
    Authorizers: fire.L{},
    Validators: fire.L{},
    Decorators: fire.L{},
    Notifiers:  fire.L{},
    // ...
}
```

The [`Authorizers`](https://godoc.org/github.com/256dpi/fire#Controller.Authorizers) are run after inferring all available data from the request and are therefore perfectly suited to do a general user authentication. The [`Validators`](https://godoc.org/github.com/256dpi/fire#Controller.Validators) are only run before creating, updating or deleting a model and are ideal to protect resources from certain actions. The [`Decorators`](https://godoc.org/github.com/256dpi/fire#Controller.Decorators) are run after the models or model have been loaded from the database or the model has been saved or updated. Finally, the [`Notifiers`]() are run before the final response is written to the client. Errors returned by the callbacks are serialize to an JSON API compliant error object and yield a status code appropriate to the class of callback.

Go on Fire ships with several built-in callbacks that implement common concerns:

- [Basic Authorizer](https://godoc.org/github.com/256dpi/fire#BasicAuthorizer)
- [Model Validator](https://godoc.org/github.com/256dpi/fire#ModelValidator)
- [Protected Fields Validator](https://godoc.org/github.com/256dpi/fire#ProtectedFieldsValidator)
- [Dependent Resources Validator](https://godoc.org/github.com/256dpi/fire#DependentResourcesValidator)
- [Referenced Resources Validator](https://godoc.org/github.com/256dpi/fire#ReferencedResourcesValidator)
- [Matching References Validator](https://godoc.org/github.com/256dpi/fire#MatchingReferencesValidator)
- [Relationship Validator](https://godoc.org/github.com/256dpi/fire#RelationshipValidator)
- [Timestamp Validator](https://godoc.org/github.com/256dpi/fire#TimestampValidator)

Custom callbacks can be created using the [`C`](https://godoc.org/github.com/256dpi/fire#C) helper:

```go
fire.C("MyAuthorizer", fire.All(), func(ctx *fire.Context) error {
    // ...
}),
```

- The first argument is the name of the callback (this is used to augment the tracing spans).
- The second argument is the matcher that decides for which operations the callback is executed.
- The third argument is the function of the callback that receives the current request context.

If returned errors from callbacks are marked as [`Safe`](https://godoc.org/github.com/256dpi/fire#Safe) or constructed using the [`E`](https://godoc.org/github.com/256dpi/fire#E) helper, the error message is serialized and returned in the JSON-API error response.

### Custom Actions

Controllers allow the definition of custom [`CollectionActions`](https://godoc.org/github.com/256dpi/fire#Controller.CollectionActions) and [`ResourceActions`](https://godoc.org/github.com/256dpi/fire#Controller.ResourceActions):

```go
postsController := &fire.Controller{
    // ...
    CollectionActions: fire.M{
    	// POST /posts/clear
    	"clear": fire.A("Clear", []string{"POST"}, 0, func(ctx *Context) error {
            // ...
        }),
    },
    ResourceActions: fire.M{
    	// GET /posts/#/avatar
    	"avatar": fire.A("Avatar", []string{"GET"}, 0, func(ctx *Context) error {
            // ...
        }),  
    },
    // ...
}
```

### Advanced Features

The `fire` package offers the following advanced features:

- [`NoList`](https://godoc.org/github.com/256dpi/fire#Controller.NoList): disables resource listing.
- [`ListLimit`](https://godoc.org/github.com/256dpi/fire#Controller.ListLimit): enforces pagination of list responses.
- [`DocumentLimit`](https://godoc.org/github.com/256dpi/fire#Controller.ListLimit): protects the API from big requests.
- [`UseTransactions`](https://godoc.org/github.com/256dpi/fire#Controller.UseTransactions): ensures atomicity using database transactions.
- [`TolerateViolations`](https://godoc.org/github.com/256dpi/fire#Controller.TolerateViolations): tolerates writes to inaccessible fields.
- [`IdempotentCreate`](https://godoc.org/github.com/256dpi/fire#Controller.IdempotentCreate): ensures idempotency of resource creations.
- [`ConsistentUpdate`](https://godoc.org/github.com/256dpi/fire#Controller.IdempotentCreate): ensures consistency of parallel resource updates.
- [`SoftDelete`](https://godoc.org/github.com/256dpi/fire#Controller.SoftDelete): soft deletes documents using a timestamp field.

## Authentication

The [`flame`](https://godoc.org/github.com/256dpi/fire/flame) package implements the OAuth2 specification and provides the "Resource Owner Password", "Client Credentials" and "Implicit" grants. The issued access and refresh tokens are [JWT](https://jwt.io) tokens and are thus able to transport custom data.

Every authenticator needs a [`Policy`](https://godoc.org/github.com/256dpi/fire/flame#Policy) that describes how the authentication is enforced. A basic policy can be created and extended using [`DefaultPolicy`](https://godoc.org/github.com/256dpi/fire/flame#DefaultPolicy):

```go
policy := flame.DefaultPolicy("a-very-long-secret")
policy.PasswordGrant = true
```

- The default policy uses the built-in [`Token`](https://godoc.org/github.com/256dpi/fire/flame#Token), [`User`](https://godoc.org/github.com/256dpi/fire/flame#User) and [`Application`](https://godoc.org/github.com/256dpi/fire/flame#Application) model and the [`DefaultGrantStrategy`](https://godoc.org/github.com/256dpi/fire/flame#DefaultGrantStrategy).
- You might want to add the indexes for the built-on models using [`AddTokenIndexes`](https://godoc.org/github.com/256dpi/fire/flame#AddTokenIndexes), [`AddApplicationIndexes`](https://godoc.org/github.com/256dpi/fire/flame#AddApplicationIndexes) and [`AddUserIndexes`](https://godoc.org/github.com/256dpi/fire/flame#AddUserIndexes).

An [`Authenticator`](https://godoc.org/github.com/256dpi/fire/flame#Authenticator) is created by specifying the policy and store:

```go
authenticator := flame.NewAuthenticator(store, policy)
```
 
After that, it can be mounted and served using the built-in http package:

```go
http.Handle("/auth/", authenticator.Endpoint("/auth/"))
```

A controller group or other endpoints can then be proctected by adding the [`Authorizer`](https://godoc.org/github.com/256dpi/fire/flame#Authenticator.Authorizer) middleware:

```
endpoint := flame.Compose(
    authenticator.Authorizer("custom-scope", true, true),
    group.Endpoint("/api/"),
)
```

More information about OAuth2 flows can be found [here](https://www.digitalocean.com/community/tutorials/an-introduction-to-oauth-2).

### Scope

The default grant strategy grants the requested scope if the client satisfies the scope. However, most applications want to grant the scope based on client types and owner roles. A custom grant strategy can be implemented by setting a different `GrantStrategy`.

The following example callback grants the `default` scope and additionally the `admin` scope if the user has the admin flag set:
 
```go
policy.GrantStrategy = func(scope oauth2.Scope, client flame.Client, ro flame.ResourceOwner) (oauth2.Scope, error) {
    list := oauth2.Scope{"default"}
    
    if ro != nil && ro.(*User).Admin {
        list = append(list, "admin")
    }

    return list, nil
}
```

### Callback

The authenticator [`Callback`](https://godoc.org/github.com/256dpi/fire/flame#Callback) can be used to authorize access to JSON API resources by requiring a scope that must have been granted:

```go
postsController := &fire.Controller{
    // ...
    Authorizers: []fire.Callback{
        flame.Callback(true, "admin"),
        // ...
    },
    // ...
}
```

- The authorizer will assign the authorized [`Token`](https://godoc.org/github.com/256dpi/fire/flame#Token) to the context using the [`AccessTokenContextKey`](https://godoc.org/github.com/256dpi/fire/flame#AccessTokenContextKey) key.

### Advanced Features

The `flame` package offers the following advanced features:

- [`ClientFilter`](https://godoc.org/github.com/256dpi/fire/flame#Policy.ClientFilter): dynamic filtering of clients based on request parameters.
- [`ResourceOwnerFilter`](https://godoc.org/github.com/256dpi/fire/flame#Policy.ResourceOwnerFilter): dynamic filtering of resource owners based on request parameters.
- [`TokenData`](https://godoc.org/github.com/256dpi/fire/flame#Policy.TokenData): custom token data.
- [`TokenMigrator`](https://godoc.org/github.com/256dpi/fire/flame#TokenMigrator): migration of tokens in queries to headers.
- [`EnsureApplication`](https://godoc.org/github.com/256dpi/fire/flame#EnsureApplication): ensure the availability of a default application.
- [`EnsureFirstUser`](https://godoc.org/github.com/256dpi/fire/flame#EnsureFirstUser): ensure the availability of a first user.

## Authorization

The [`ash`](https://godoc.org/github.com/256dpi/fire/ash) package implements a simple framework for declaratively define authorization of resources.

Authorization rules are defined using a [`Strategy`](https://godoc.org/github.com/256dpi/fire/ash#Strategy) that can be converted into a callback using the [`C`](https://godoc.org/github.com/256dpi/fire/ash#C) helper:

```go
postsController := &fire.Controller{
    // ...
    Authorizers: fire.L{
        ash.C(&ash.Strategy{
        	// ...
        	Read: ash.L{},
            Write: ash.L{},
            // ...
        }),
    },
    // ...
}
```

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
