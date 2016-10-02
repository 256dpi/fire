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

## Example

An example application that uses the fire framework to build an JSON API that is consumed by an Ember Application can be found here: <https://github.com/gonfire/example>.

## Installation

Get the package using the go tool:

```bash
$ go get -u github.com/gonfire/fire
```

## License

The MIT License (MIT)

Copyright (c) 2016 Joël Gähwiler
