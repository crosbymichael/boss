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

ENV CONTAINERD_VERSION 27cafbb3b81481347eace9bae2751ec0220cc02c
RUN git clone https://github.com/crosbymichael/containerd /go/src/github.com/containerd/containerd
RUN git clone https://github.com/opencontainers/runc /go/src/github.com/opencontainers/runc

WORKDIR /go/src/github.com/containerd/containerd
RUN git checkout ${CONTAINERD_VERSION}

RUN rsync -au /go/src/github.com/containerd/containerd/vendor/ /go/src/ && \
	rm -rf /go/src/github.com/containerd/containerd/vendor/

RUN ./script/setup/install-protobuf
RUN ./script/setup/install-runc
RUN make

FROM containerd as boss

ADD . /go/src/github.com/crosbymichael/boss
RUN	rm -rf /go/src/github.com/crosbymichael/boss/vendor/github.com/containerd/ && \
	rsync -au --ignore-existing /go/src/github.com/crosbymichael/boss/vendor/ /go/src/ && \
	rm -rf /go/src/github.com/crosbymichael/boss/vendor/

WORKDIR /go/src/github.com/crosbymichael/boss

RUN make && make plugin

FROM scratch

COPY --from=containerd /go/src/github.com/containerd/containerd/bin/* /bin/
COPY --from=containerd /usr/local/sbin/runc /sbin/
COPY --from=boss /go/src/github.com/crosbymichael/boss/bin/boss /bin/
COPY --from=boss /go/src/github.com/crosbymichael/boss/boss-linux-amd64.so /plugins/
