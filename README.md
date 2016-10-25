<img src="http://joel-github-static.s3.amazonaws.com/gonfire/logo.png" alt="Logo" width="256"/>

# Go on Fire

[![Build Status](https://travis-ci.org/gonfire/fire.svg?branch=master)](https://travis-ci.org/gonfire/fire)
[![Coverage Status](https://coveralls.io/repos/github/gonfire/fire/badge.svg?branch=master)](https://coveralls.io/github/gonfire/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/gonfire/fire?status.svg)](http://godoc.org/github.com/gonfire/fire)
[![Release](https://img.shields.io/github/release/gonfire/fire.svg)](https://github.com/gonfire/fire/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/gonfire/fire)](http://goreportcard.com/report/gonfire/fire)

**An idiomatic micro-framework for building Ember.js compatible APIs with Go.**

[Go on Fire](https://gonfire.org) is built on top of the wonderful and already built-in [http](golang http client library) package, implements the [JSON API](http://jsonapi.org) specification through the dedicated [jsonapi](https://github.com/gonfire/jsonapi) library, uses the very stable [mgo](https://github.com/go-mgo/mgo) [MongoDB](https://www.mongodb.com) driver for persisting resources and leverages the dedicated [oauth2](https://github.com/gonfire/oauth2) library to provide out of the box support for [OAuth2](https://oauth.net/2/) authentication.

The deliberate and tight integration of these components provides a very simple and extensible set of abstractions for rapidly building backend services for websites that use [Ember.js](http://emberjs.com) as their frontend framework. Of course it can also be used in conjunction with any other single page application framework or as a backend for native mobile applications.

An example application that uses Go on Fire to build an OAuth2 secured JSON API that is consumed by an Ember.js application can be found here: [github.com/gonfire/example](https://github.com/gonfire/example).

To quickly get started with building an API with Go on Fire follow the [quickstart guide](http://gonfire.org/#quickstart), read the detailed sections in this documentation and refer to the [package documentation](https://godoc.org/github.com/gonfire/fire) for more detailed information on the used types and methods. 

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
