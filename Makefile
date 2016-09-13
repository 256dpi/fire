all: test fmt vet lint

test:
	go test -cover .
	go test -cover ./model
	go test -cover ./components

vet:
	go vet .
	go vet ./model
	go vet ./components
	go vet ./examples/app

fmt:
	go fmt .
	go fmt ./model
	go fmt ./components
	go fmt ./examples/app

lint:
	golint .
	golint ./model
	golint ./components
	golint ./examples/app

err:
	errcheck -ignoretests -asserts .

toc:
	doctoc README.md
