all: fmt vet lint

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
