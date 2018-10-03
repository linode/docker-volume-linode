
# Build Arguments
TRAVIS_BRANCH ?= test
TRAVIS_BUILD_NUMBER ?= 9999

# Deploy Arguments
DOCKER_USERNAME ?= xxxxx
DOCKER_PASSWORD ?= xxxxx

# Test Arguments
TEST_TOKEN ?= xyz
TEST_REGION ?= xyz
TEST_LABEL ?= xyz

GOPATH=$(shell go env GOPATH)

# e.g: docker-volume-linode:rootfs.30
PLUGIN_NAME_ROOTFS=docker-volume-linode:rootfs.${TRAVIS_BUILD_NUMBER}

# e.g: docker-volume-linode:master.30
# e.g: docker-volume-linode:v1.1.30
PLUGIN_NAME=linode/docker-volume-linode:${TRAVIS_BRANCH}.${TRAVIS_BUILD_NUMBER}
PLUGIN_NAME_LATEST=linode/docker-volume-linode:latest

PLUGIN_DIR=plugin-contents-dir

all: clean build

deploy: build
	# Login to docker
	@echo '${DOCKER_PASSWORD}' | docker login -u "${DOCKER_USERNAME}" --password-stdin
	# Push images
	docker plugin push ${PLUGIN_NAME}
	docker plugin push ${PLUGIN_NAME_LATEST}

build: $(PLUGIN_DIR)
	# load plugin with versionied tag
	docker plugin rm -f ${PLUGIN_NAME} 2>/dev/null || true
	docker plugin create ${PLUGIN_NAME} ./$(PLUGIN_DIR)
	# load plugin with `latest` tag
	docker plugin rm -f ${PLUGIN_NAME_LATEST} 2>/dev/null || true
	docker plugin create ${PLUGIN_NAME_LATEST} ./$(PLUGIN_DIR)

$(PLUGIN_DIR): $(GOPATH)/bin/dep *.go Dockerfile
	# compile
	dep ensure
	docker build --no-cache -q -t ${PLUGIN_NAME_ROOTFS} .
	# assemble
	mkdir -p ./$(PLUGIN_DIR)/rootfs
	docker create --name tmp  ${PLUGIN_NAME_ROOTFS}
	docker export tmp | tar -x -C ./$(PLUGIN_DIR)/rootfs
	cp config.json ./$(PLUGIN_DIR)/
	docker rm -vf tmp

# Run Integration Tests
#   Requires TEST_* Variables to be set
test: test-pre-check \
	build \
	test-setup \
	test-create-volume-50 \
	test-rm-volume-50 \
	test-create-volume \
	test-use-volume \
	clean-volumes

test-create-volume:
	docker volume create -d $(PLUGIN_NAME) test-volume-default-size

test-create-volume-50:
	docker volume create -d $(PLUGIN_NAME) -o size=50 test-volume-50g

test-rm-volume-50:
	docker volume rm test-volume-50g

test-use-volume:
	docker run --rm -i -v test-volume-default-size:/mnt busybox touch /mnt/abc.txt
	docker run --rm -i -v test-volume-default-size:/mnt busybox test -f /mnt/abc.txt || false

test-pre-check:
	@if [ "${TEST_TOKEN}" = "xyz" ] || [ "${TEST_REGION}" = "xyz" ] || [ "${TEST_LABEL}" = "xyz" ] ; then \
		echo -en "#############################\nYou must set TEST_* Variables\n#############################\n"; exit 1; fi

test-setup:
	@docker plugin set $(PLUGIN_NAME) LINODE_TOKEN=${TEST_TOKEN} LINODE_REGION=${TEST_REGION} LINODE_LABEL=${TEST_LABEL}
	docker plugin enable  $(PLUGIN_NAME)

check: $(GOPATH)/bin/dep
	# Tools
	go get -u github.com/tsenart/deadcode
	go get -u github.com/kisielk/errcheck
	go get -u golang.org/x/lint/golint
	# Run Code Tests
	dep ensure
	go vet
	errcheck
	golint
	deadcode

unit-test: $(GOPATH)/bin/dep
	dep ensure
	go test

$(GOPATH)/bin/dep:
	go get -u github.com/golang/dep/cmd/dep

.PHONY clean:
	rm -fr $(PLUGIN_DIR)

clean-volumes:
	docker volume ls -q | grep 'test-' | xargs docker volume rm
clean-installed-plugins:
	docker plugin ls | grep linode | grep -v ID | awk '{print $$1}' | xargs docker plugin rm -f

