all: fmt vet lint test

fmt:
	go fmt .
	go fmt ./model
	go fmt ./jsonapi
	go fmt ./auth
	go fmt ./components

vet:
	go vet .
	go vet ./model
	go vet ./jsonapi
	go vet ./auth
	go vet ./components

lint:
	golint .
	golint ./model
	golint ./jsonapi
	go vet ./auth
	golint ./components

setup:
	mkdir -p .test/assets
	echo '<h1>Hello</h1>' > .test/assets/index.html
	mkdir -p .test/tls
	openssl req -x509 -newkey rsa:4096 -keyout .test/tls/key.pem -out .test/tls/cert.pem -days 90 -nodes -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=www.example.com"

test: setup
	go test -cover .
	go test -cover ./model
	go test -cover ./jsonapi
	go test -cover ./components
