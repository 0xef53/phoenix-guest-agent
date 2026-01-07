PROJECT_NAME := phoenix-guest-agent
PROJECT_REPO := github.com/0xef53/$(PROJECT_NAME)

GOLANG_IMAGE := golang:1.24-bullseye
DEVTOOLS_IMAGE := 0xef53/devtools:debian-bullseye

CWD := $(shell pwd)

ifeq (,$(wildcard /etc/debian_version))
    SYSTEMD_UNITDIR ?= /usr/lib/systemd/system
else
    SYSTEMD_UNITDIR ?= /lib/systemd/system
endif

DOCKER_BUILD_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(PROJECT_NAME)_pkg:/go/pkg \
    -v $(CWD):/go/$(PROJECT_NAME) \
    -v $(CWD)/scripts/build.sh:/usr/local/bin/build.sh \
    -e GOBIN=/go/$(PROJECT_NAME)/bin \
    --entrypoint build.sh

DOCKER_TESTS_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(PROJECT_NAME)_pkg:/go/pkg \
    -v $(CWD):/go/$(PROJECT_NAME)

DOCKER_PB_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(CWD):/go/$(PROJECT_NAME)

protofiles_grpc = \
    types/v2/agent.proto \
    services/agent/v2/agent.proto \
	services/system/v2/system.proto

DOCKER_DEB_ARGS := \
    -w /root/source \
    -v $(CWD):/root/source:ro \
    -v $(CWD)/packages:/root/source/packages \
    -v $(CWD)/scripts/build-deb.sh:/usr/local/bin/build-deb.sh \
    -e PROJECT_NAME=$(PROJECT_NAME) \
    --entrypoint build-deb.sh

binaries = \
    bin/agent bin/client

.PHONY: all build clean protobufs $(proto_files)

all: build

$(binaries):
	@echo "##########################"
	@echo "#  Building binaries     #"
	@echo "##########################"
	@echo
	install -d bin
	docker run --rm -it $(DOCKER_BUILD_ARGS) $(GOLANG_IMAGE)
	@echo
	@echo "==================="
	@echo "Successfully built:"
	ls -lh bin/
	@echo

build: $(binaries)

tests:
	@echo "##########################"
	@echo "#  Running tests         #"
	@echo "##########################"
	@echo
	docker run --rm -i $(DOCKER_TESTS_ARGS) $(GOLANG_IMAGE) go test ./...
	@echo
	@echo

protobufs:
	docker run --rm -it $(DOCKER_PB_ARGS) 0xef53/go-proto-compiler:v3.18 \
		--proto_path api \
		--go_opt "plugins=grpc,paths=source_relative" \
		--go_out ./api \
		$(protofiles_grpc)
	scripts/fix-proto-names.sh $(shell find api/ -type f -name '*.pb.go')

install: $(binaries)
	install -d $(DESTDIR)/usr/bin $(DESTDIR)/usr/lib/$(PROJECT_NAME)
	install -d $(DESTDIR)$(SYSTEMD_UNITDIR)
	cp -t $(DESTDIR)$(SYSTEMD_UNITDIR) contrib/$(PROJECT_NAME).service
	@echo

deb-package: $(binaries)
	@echo "##########################"
	@echo "#  Building deb package  #"
	@echo "##########################"
	@echo
	install -d packages
	docker run --rm -i $(DOCKER_DEB_ARGS) $(DEVTOOLS_IMAGE)
	@echo
	@echo "==================="
	@echo "Successfully built:"
	@find packages -type f -name '*.deb' -printf "%p\n"
	@echo

clean:
	rm -Rvf bin packages vendor