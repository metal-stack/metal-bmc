BINARY := ipmi-catcher
MAINMODULE := github.com/metal-stack/ipmi-catcher
COMMONDIR := $(or ${COMMONDIR},../builder)
DOCKER_TAG := $(or ${GITHUB_TAG_NAME}, latest)

include $(COMMONDIR)/Makefile.inc

release:: test all;

.PHONY: fmt
fmt:
	GO111MODULE=off go fmt ./...

.PHONY: dockerimage
dockerimage:
	docker build -t metalstack/ipmi-catcher:${DOCKER_TAG} .

.PHONY: dockerpush
dockerpush:
	docker push metalstack/ipmi-catcher:${DOCKER_TAG}