all: test fmt vet lint

test:
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
	go vet ./examples/app

fmt:
	go fmt .
	go fmt ./model
	go fmt ./jsonapi
	go fmt ./oauth2
	go fmt ./components
	go fmt ./examples/app

lint:
	golint .
	golint ./model
	golint ./jsonapi
	golint ./oauth2
	golint ./components
	golint ./examples/app

err:
	errcheck -ignoretests -asserts .

toc:
	doctoc README.md
