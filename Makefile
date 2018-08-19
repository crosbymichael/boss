PACKAGES=$(shell go list ./... | grep -v /vendor/)
REVISION=$(shell git rev-parse HEAD)
GO_LDFLAGS=-s -w -X github.com/crosbymichael/boss.Version=$(REVISION)

all:
	go build -v -ldflags '${GO_LDFLAGS}'

install:
	@install boss /usr/local/bin/boss

protos:
	protobuild --quiet ${PACKAGES}
