<img src="https://joel-github-static.s3.amazonaws.com/gonfire/logo.png" alt="Logo" width="256"/>

# Go on Fire

[![Test](https://github.com/256dpi/fire/actions/workflows/test.yml/badge.svg)](https://github.com/256dpi/fire/actions/workflows/test.yml)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)

**An idiomatic micro-framework for building Ember.js compatible APIs with Go.**

## Introduction

**Go on Fire** is built on top of the wonderful built-in [http](https://golang.org/pkg/net/http) package, implements the [JSON API](http://jsonapi.org) specification through the dedicated [jsonapi](https://github.com/256dpi/jsonapi) library, uses the official [mongo](https://github.com/mongodb/mongo-go-driver) driver for persisting resources with [MongoDB](https://www.mongodb.com), and leverages the dedicated [oauth2](https://github.com/256dpi/oauth2) library to provide out-of-the-box support for [OAuth2](https://oauth.net/2/) authentication using [JWT](https://jwt.io) tokens. Additionally, it provides packages for request authorization, asynchronous job processing, and WebSocket-based event sourcing.  

The deliberate and tight integration of these components provides a very simple and extensible set of abstractions for rapidly building backend services for websites that use [Ember.js](http://emberjs.com) as their frontend framework. Of course, it can also be used in conjunction with any other single-page application framework or as a backend for native mobile applications.

To get started with building with Go on Fire, refer to the [package documentation](https://godoc.org/github.com/256dpi/fire) for more detailed information on the types and methods. Also, have a look at the [example](https://github.com/256dpi/fire/tree/master/example) application that uses most Go on Fire features.

## Features

Go on Fire ships with built-in support for various features to provide a complete toolkit for ambitious projects:

- Declarative definition of models and resource controllers.
- Custom group, collection, and resource actions.
- Built-in validators incl. automatic relationship validation.
- Callback-based plugin system for easy extendability.
- Integrated asynchronous and distributed job processing system.
- Event sourcing via WebSockets and SSE.
- Declarative authentication and authorization framework.
- Integrated OAuth2 authenticator and authorizer.
- Support for tracing via [opentracing](https://opentracing.io).

## Installation

To get started, install the package using the go tool:

```bash
$ go get -u github.com/256dpi/fire
```

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
