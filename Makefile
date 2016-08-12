PKGS=$(shell glide nv)

all: fmt vet lint err

vet:
	go vet $(PKGS)

fmt:
	go fmt $(PKGS)

lint:
	golint $(PKGS)

err:
	errcheck -ignoretests -asserts $(PKGS)
