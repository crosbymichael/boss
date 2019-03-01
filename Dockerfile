FROM golang:1.12 as containerd

RUN apt-get update && \
	apt-get install -y rsync autoconf automake g++ libtool unzip \
	btrfs-tools gcc git libseccomp-dev make xfsprogs

RUN git clone https://github.com/containerd/containerd /go/src/github.com/containerd/containerd
ADD . /go/src/github.com/crosbymichael/boss

RUN rsync -au /go/src/github.com/containerd/containerd/vendor/ /go/src/ && \
	rm -rf /go/src/github.com/containerd/containerd/vendor/

RUN rsync -au --ignore-existing /go/src/github.com/crosbymichael/boss/vendor/ /go/src/ && \
	rm -rf /go/src/github.com/crosbymichael/boss/vendor/

WORKDIR /go/src/github.com/containerd/containerd
RUN ./script/setup/install-protobuf

FROM containerd AS build

WORKDIR /go/src/github.com/containerd/containerd
RUN make
RUN go build -o bin/boss-linux-amd64.so -buildmode=plugin github.com/crosbymichael/boss/plugin

FROM scratch

COPY --from=build /go/src/github.com/containerd/containerd/bin/* /bin/
