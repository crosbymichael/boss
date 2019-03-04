FROM golang:1.12 as containerd

RUN apt-get update && \
	apt-get install -y \
	rsync \
	autoconf \
	automake \
	g++ \
	libtool \
	unzip \
	btrfs-tools \
	gcc \
	git \
	libseccomp-dev \
	make \
	xfsprogs

ENV CONTAINERD_VERSION 30b6f460b96137947b3de5ec92134d56cb763708
RUN git clone https://github.com/containerd/containerd /go/src/github.com/containerd/containerd

RUN rsync -au /go/src/github.com/containerd/containerd/vendor/ /go/src/ && \
	rm -rf /go/src/github.com/containerd/containerd/vendor/

WORKDIR /go/src/github.com/containerd/containerd
RUN git checkout ${CONTAINERD_VERSION}
RUN ./script/setup/install-protobuf
RUN make

FROM containerd AS boss

ADD . /go/src/github.com/crosbymichael/boss
RUN rsync -au --ignore-existing /go/src/github.com/crosbymichael/boss/vendor/ /go/src/ && \
	rm -rf /go/src/github.com/crosbymichael/boss/vendor/

WORKDIR /go/src/github.com/crosbymichael/boss

RUN make && make plugin

FROM scratch

COPY --from=containerd /go/src/github.com/containerd/containerd/bin/* /bin/
COPY --from=boss /go/src/github.com/crosbymichael/boss/boss /bin/
COPY --from=boss /go/src/github.com/crosbymichael/boss/boss-linux-amd64.so /bin/
