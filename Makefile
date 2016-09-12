all: test fmt vet lint

test:
	go test -cover .

vet:
	go vet .
	go vet ./examples/echo
	go vet ./examples/app

fmt:
	go fmt .
	go fmt ./examples/echo
	go fmt ./examples/app

lint:
	golint .
	golint ./examples/echo
	golint ./examples/app

err:
	errcheck -ignoretests -asserts .

toc:
	doctoc README.md
