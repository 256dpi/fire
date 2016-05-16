# fire

[![Circle CI](https://img.shields.io/circleci/project/256dpi/fire.svg)](https://circleci.com/gh/256dpi/fire)
[![Coverage Status](https://coveralls.io/repos/256dpi/fire/badge.svg?branch=master&service=github)](https://coveralls.io/github/256dpi/fire?branch=master)
[![GoDoc](https://godoc.org/github.com/256dpi/fire?status.svg)](http://godoc.org/github.com/256dpi/fire)
[![Release](https://img.shields.io/github/release/256dpi/fire.svg)](https://github.com/256dpi/fire/releases)
[![Go Report Card](http://goreportcard.com/badge/256dpi/fire)](http://goreportcard.com/report/256dpi/fire)

**A small and opinionated framework for Go providing an Ember Data compatible JSON API.**

Fire is built on [api2go](https://github.com/manyminds/api2go), uses the [mgo](https://github.com/go-mgo/mgo) MongoDB driver for persisting resources and plays well with the [gin](https://github.com/gin-gonic/gin) framework. The tight integration of these components provides a very simple API for rapidly building JSON API services for your Ember projects.
