#!/bin/bash

# e.g: docker-volume-linode:rootfs.30
export PLUGIN_NAME_ROOTFS=docker-volume-linode:rootfs.${TRAVIS_BUILD_NUMBER}

# e.g: docker-volume-linode:master.30
# e.g: docker-volume-linode:v1.1.30
export PLUGIN_NAME=libgolang/docker-volume-linode:${TRAVIS_BRANCH}.${TRAVIS_BUILD_NUMBER}

# Build Step
build () {
	# compile
	go get -u github.com/golang/dep/cmd/dep
	dep ensure
	docker build --no-cache -q -t ${PLUGIN_NAME_ROOTFS} .

	# create plugin
	mkdir -p ./plugin/rootfs
	docker create --name tmp  ${PLUGIN_NAME_ROOTFS}
	docker export tmp | tar -x -C ./plugin/rootfs
	cp config.json ./plugin/
	docker rm -vf tmp
	docker plugin rm -f ${PLUGIN_NAME} || true
	docker plugin create ${PLUGIN_NAME} ./plugin
}


# Deploy Step
deploy() {
	# Login to docker
	echo "$DOCKER_PASSWORD" | docker login -u libgolang --password-stdin

	# Push image
	docker plugin push ${PLUGIN_NAME}
}
