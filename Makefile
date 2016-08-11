FILES=$(shell go list ./... | grep -v '/vendor/')

vet:
	go vet $(FILES)

fmt:
	go fmt $(FILES)
