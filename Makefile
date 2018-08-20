PACKAGES=$(shell go list ./... | grep -v /vendor/)
REVISION=$(shell git rev-parse HEAD)
GO_LDFLAGS=-s -w -X github.com/crosbymichael/boss/version.Version=$(REVISION)

all:
	go build -v -ldflags '${GO_LDFLAGS}'

static:
	CGO_ENALBED=0 go build -v -ldflags '${GO_LDFLAGS} -extldflags "-static"'

install:
	@install boss /usr/local/bin/boss

protos:
	protobuild --quiet ${PACKAGES}
