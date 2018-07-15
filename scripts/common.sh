#!/bin/bash

# e.g: docker-volume-linode:rootfs.30
export PLUGIN_NAME_ROOTFS=docker-volume-linode:rootfs.${TRAVIS_BUILD_NUMBER}

# e.g: docker-volume-linode:master.30
# e.g: docker-volume-linode:v1.1.30
export PLUGIN_NAME=libgolang/docker-volume-linode:${TRAVIS_BRANCH}.${TRAVIS_BUILD_NUMBER}
export PLUGIN_NAME_LATEST=libgolang/docker-volume-linode:latest

# Build Step
build () {
	compile
	assemble-plugin-dir
	create-plugin-from-dir ${PLUGIN_NAME}
}

# Build Latest Step
build-latest () {
	compile
	assemble-plugin-dir
	create-plugin-from-dir ${PLUGIN_NAME_LATEST}
}

# Deploy Step
deploy () {
	# Login to docker
	echo "$DOCKER_PASSWORD" | docker login -u libgolang --password-stdin

	# Push image
	docker plugin push ${PLUGIN_NAME}
}

# Deploy Latest Tag Step
deploy-latest () {
	# Login to docker
	echo "$DOCKER_PASSWORD" | docker login -u libgolang --password-stdin

	# Push image
	docker plugin push ${PLUGIN_NAME_LATEST}
}

compile () {
	go get -u github.com/golang/dep/cmd/dep
	dep ensure
	docker build --no-cache -q -t ${PLUGIN_NAME_ROOTFS} .
}

assemble-plugin-dir () {
	# create plugin
	mkdir -p ./plugin/rootfs
	docker create --name tmp  ${PLUGIN_NAME_ROOTFS}
	docker export tmp | tar -x -C ./plugin/rootfs
	cp config.json ./plugin/
	docker rm -vf tmp
}

create-plugin-from-dir () {
	docker plugin rm -f $1 || true
	docker plugin create $1 ./plugin
}
