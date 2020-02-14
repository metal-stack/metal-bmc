.ONESHELL:
BINARY := ipmi-catcher
COMMONDIR := $(or ${COMMONDIR},../common)
DOCKER_TAG := $(or ${GITHUB_TAG_NAME}, latest)

include $(COMMONDIR)/Makefile.inc

release:: clean-local-dirs generate-client test all;

.PHONY: clean-local-dirs
clean-local-dirs:
	rm -rf metal-api
	mkdir metal-api

.PHONY: clean-client
clean-client: clean-local-dirs
	cp ../metal-api/spec/metal-api.json metal-api.json

.PHONY: fmt
fmt:
	GO111MODULE=off go fmt ./...

.PHONY: generate-client
generate-client: clean-local-dirs fmt
	GO111MODULE=off swagger generate client --target=metal-api -f metal-api.json --skip-validation

.PHONY: dockerimage
dockerimage:
	docker build -t metalstack/ipmi-catcher:${DOCKER_TAG} .

.PHONY: dockerpush
dockerpush:
	docker push metalstack/ipmi-catcher:${DOCKER_TAG}