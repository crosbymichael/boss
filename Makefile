PACKAGES=$(shell go list ./... | grep -v /vendor/)

all:
	go build

install:
	@install boss /usr/local/bin/boss

protos:
	protobuild --quiet ${PACKAGES}
