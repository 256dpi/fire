all: fmt vet lint test

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html
	mkdir -p .test/tls
	openssl req -x509 -newkey rsa:4096 -keyout .test/tls/key.pem -out .test/tls/cert.pem -days 90 -nodes -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"

test: setup
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

fmt:
	go fmt .
	go fmt ./model
	go fmt ./jsonapi
	go fmt ./oauth2
	go fmt ./components

lint:
	golint .
	golint ./model
	golint ./jsonapi
	golint ./oauth2
	golint ./components

err:
	errcheck -ignoretests -asserts .
