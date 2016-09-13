all: test fmt vet lint

test:
	go test -cover .
	go test -cover ./model

vet:
	go vet .
	go vet ./model
	go vet ./examples/echo
	go vet ./examples/app

fmt:
	go fmt .
	go fmt ./model
	go fmt ./examples/echo
	go fmt ./examples/app

lint:
	golint .
	golint ./model
	golint ./examples/echo
	golint ./examples/app

err:
	errcheck -ignoretests -asserts .

toc:
	doctoc README.md
