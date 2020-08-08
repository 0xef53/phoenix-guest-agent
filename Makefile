PROJECT_NAME := phoenix-guest-agent
PROJECT_REPO := github.com/0xef53/$(PROJECT_NAME)

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

DOCKER_PB_ARGS := \
    -w /go/$(PROJECT_NAME) \
    -v $(CWD):/go/$(PROJECT_NAME)

binaries = bin/agent bin/client

.PHONY: all build protobufs clean

all: build

$(binaries):
	@echo "##########################"
	@echo "#  Building binaries     #"
	@echo "##########################"
	@echo
	install -d bin
	docker run --rm -i $(DOCKER_BUILD_ARGS) golang:latest
	@echo
	@echo "==================="
	@echo "Successfully built:"
	ls -lh bin/
	@echo

build: $(binaries)

protobufs:
	docker run --rm -i $(DOCKER_PB_ARGS) 0xef53/go-proto-compiler:latest \
		--proto_path protobufs \
		--proto_path /go/src/github.com/gogo/googleapis \
		--proto_path /go/src \
		--gogofast_out=plugins=grpc,paths=source_relative,\
	Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
	Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
	Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types:\
	./protobufs \
		protobufs/agent/agent.proto

install: $(binaries)
	install -d $(DESTDIR)/usr/bin $(DESTDIR)/usr/lib/$(PROJECT_NAME)
	install -d $(DESTDIR)/lib/systemd/system
	cp -t $(DESTDIR)$(SYSTEMD_UNITDIR) $(PROJECT_NAME).service
	@echo

clean:
	rm -Rvf bin packages vendor
