FROM golang:1.11.5-alpine as builder
ENV GO111MODULE=on
ARG VERSION=0
COPY . /go/src/github.com/linode/docker-volume-linode
WORKDIR /go/src/github.com/linode/docker-volume-linode
RUN apk update && apk add git \
    && apk add --no-cache --virtual .build-deps gcc libc-dev \
    && go install --ldflags "-extldflags '-static' -X main.VERSION=$VERSION" \
    && apk del .build-deps
CMD ["/go/bin/docker-volume-linode"]

FROM alpine
COPY --from=builder /go/bin/docker-volume-linode .
RUN apk update && apk add ca-certificates e2fsprogs xfsprogs btrfs-progs util-linux
CMD ["./docker-volume-linode"]
