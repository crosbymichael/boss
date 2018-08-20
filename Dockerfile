FROM golang:1.10.3-alpine3.8 as builder

RUN apk add --no-cache git alpine-sdk make

ADD . /go/src/github.com/crosbymichael/boss
WORKDIR /go/src/github.com/crosbymichael/boss

RUN make static

FROM scratch

COPY --from=builder /go/src/github.com/crosbymichael/boss/boss /boss
