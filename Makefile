all: fmt vet lint

vet:
	go vet .
	go vet ./examples/echo

fmt:
	go fmt .
	go fmt ./examples/echo

lint:
	golint .
	golint ./examples/echo

err:
	errcheck -ignoretests -asserts .

toc:
	doctoc README.md
