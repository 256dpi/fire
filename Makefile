all: fmt vet lint test

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html

test: setup
	go test -cover .
	go test -cover ./model
	go test -cover ./jsonapi
	go test -cover ./oauth2
	go test -cover ./components

vet:
	go vet .
	go vet ./model
	go vet ./jsonapi
	go vet ./oauth2
	go vet ./components
	go vet ./example

fmt:
	go fmt .
	go fmt ./model
	go fmt ./jsonapi
	go fmt ./oauth2
	go fmt ./components
	go fmt ./example

lint:
	golint .
	golint ./model
	golint ./jsonapi
	golint ./oauth2
	golint ./components
	golint ./example

err:
	errcheck -ignoretests -asserts .

toc:
	doctoc README.md
