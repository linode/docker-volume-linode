FROM golang:1.11.5-alpine as builder
COPY . /go/src/github.com/linode/docker-volume-linode
WORKDIR /go/src/github.com/linode/docker-volume-linode
RUN set -ex \
    && apk update && apk add git \
    && apk add --no-cache --virtual .build-deps \
    gcc libc-dev \
    && go install --ldflags '-extldflags "-static"' \
    && apk del .build-deps
CMD ["/go/bin/docker-volume-linode"]

FROM alpine
COPY --from=builder /go/bin/docker-volume-linode .
RUN apk update && apk add ca-certificates e2fsprogs
CMD ["./docker-volume-linode"]
