PACKAGES=$(shell go list ./... | grep -v /vendor/)
REVISION=$(shell git rev-parse HEAD)
GO_LDFLAGS=-s -w -X github.com/crosbymichael/boss/version.Version=$(REVISION)

all:
	rm -f bin/*
	go build -o bin/boss -v -ldflags '${GO_LDFLAGS}'
	go build -o bin/boss-systemd -v -ldflags '${GO_LDFLAGS}' github.com/crosbymichael/boss/boss-systemd
	go build -o bin/boss-network -v -ldflags '${GO_LDFLAGS}' github.com/crosbymichael/boss/boss-network

static:
	CGO_ENALBED=0 go build -v -ldflags '${GO_LDFLAGS} -extldflags "-static"'

install:
	@install bin/* /usr/local/bin/

FORCE:

plugin: FORCE
	go build -o boss-linux-amd64.so -v -buildmode=plugin github.com/crosbymichael/boss/plugin/

protos:
	protobuild --quiet ${PACKAGES}
