all: fmt vet lint test

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golint ./...

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html

test:
	go test -cover ./...
