FROM golang:1.18.3-alpine as builder
ENV GO111MODULE=on
ARG VERSION=0
COPY . /docker-volume-linode
WORKDIR /docker-volume-linode
RUN apk update && apk add git \
    && apk add --no-cache --virtual .build-deps gcc libc-dev \
    && go install --ldflags "-extldflags '-static' -X main.VERSION=$VERSION" \
    && apk del .build-deps

FROM alpine
COPY --from=builder /go/bin/docker-volume-linode .
RUN apk update && apk add ca-certificates e2fsprogs xfsprogs btrfs-progs util-linux
CMD ["./docker-volume-linode"]
