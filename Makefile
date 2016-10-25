all: fmt vet lint test

fmt:
	go fmt .
	go fmt ./auth

vet:
	go vet .
	go vet ./auth

lint:
	golint .
	go vet ./auth

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html

test: setup
	go test -cover .
	go test -cover ./auth
