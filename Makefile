all: fmt vet lint test

fmt:
	go fmt .
	go fmt ./jsonapi
	go fmt ./auth
	go fmt ./components

vet:
	go vet .
	go vet ./jsonapi
	go vet ./auth
	go vet ./components

lint:
	golint .
	golint ./jsonapi
	go vet ./auth
	golint ./components

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html

test: setup
	go test -cover .
	go test -cover ./jsonapi
	go test -cover ./components
