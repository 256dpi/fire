all: fmt vet lint test

fmt:
	go fmt .
	go fmt ./auth
	go fmt ./tools

vet:
	go vet .
	go vet ./auth
	go vet ./tools

lint:
	golint .
	golint ./auth
	golint ./tools

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html

test:
	go test -cover .
	go test -cover ./auth
	go test -cover ./tools
