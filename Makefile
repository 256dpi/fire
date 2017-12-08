all: fmt vet lint test

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	@$(foreach pkg, $(shell glide novendor), golint $(pkg);)

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html

test:
	go test ./...
