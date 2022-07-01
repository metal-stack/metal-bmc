.ONESHELL:
SHA := $(shell git rev-parse --short=8 HEAD)
GITVERSION := $(shell git describe --long --all)
BUILDDATE := $(shell date -Iseconds)
VERSION := $(or ${VERSION},devel)

all: test metal-bmc

.PHONY: test
test:
	go test -v ./...

.PHONY: metal-bmc
metal-bmc:
	go build \
		-trimpath \
		-tags netgo \
		-ldflags "-X 'github.com/metal-stack/v.Version=$(VERSION)' \
				  -X 'github.com/metal-stack/v.Revision=$(GITVERSION)' \
				  -X 'github.com/metal-stack/v.GitSHA1=$(SHA)' \
				  -X 'github.com/metal-stack/v.BuildDate=$(BUILDDATE)'" \
		-o bin/metal-bmc \
		main.go
	strip bin/metal-bmc
