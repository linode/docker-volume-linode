
# Build Arguments
REPO_SLUG ?= linode/docker-volume-linode

# Deploy Arguments
DOCKER_USERNAME ?= xxxxx
DOCKER_PASSWORD ?= xxxxx

# Test Arguments
TEST_TOKEN ?= xyz
TEST_LABEL ?= xyz

GOPATH=$(shell go env GOPATH)

PLUGIN_VERSION=$(shell git describe --tags --always --abbrev=0)

PLUGIN_NAME_ROOTFS=docker-volume-linode:rootfs.${PLUGIN_VERSION}
PLUGIN_NAME=${REPO_SLUG}:${PLUGIN_VERSION}
PLUGIN_NAME_LATEST=${REPO_SLUG}:latest

PLUGIN_DIR=plugin-contents-dir

export GO111MODULE=on

all: clean build

deploy:
	# workaround for plugin
	docker plugin rm -f ${PLUGIN_NAME} 2>/dev/null || true
	docker plugin create ${PLUGIN_NAME} ./$(PLUGIN_DIR)
	docker plugin push ${PLUGIN_NAME}
	docker plugin rm -f ${PLUGIN_NAME} 2>/dev/null || true

	# load plugin with `latest` tag
	docker plugin rm -f ${PLUGIN_NAME_LATEST} 2>/dev/null || true
	docker plugin create ${PLUGIN_NAME_LATEST} ./$(PLUGIN_DIR)
	docker plugin push ${PLUGIN_NAME_LATEST}
	docker plugin rm -f ${PLUGIN_NAME_LATEST} 2>/dev/null || true

docker-login:
	# Login to docker
	echo '${DOCKER_PASSWORD}' | docker login -u "${DOCKER_USERNAME}" --password-stdin

build: $(PLUGIN_DIR)
	# load plugin with versionied tag
	docker plugin rm -f ${PLUGIN_NAME} 2>/dev/null || true
	docker plugin create ${PLUGIN_NAME} ./$(PLUGIN_DIR)
	# load plugin with `latest` tag
	docker plugin rm -f ${PLUGIN_NAME} 2>/dev/null || true
	docker plugin rm -f ${PLUGIN_NAME_LATEST} 2>/dev/null || true
	docker plugin create ${PLUGIN_NAME_LATEST} ./$(PLUGIN_DIR)

$(PLUGIN_DIR): *.go Dockerfile
	# compile
	docker build --build-arg VERSION="${PLUGIN_VERSION}" --no-cache -q -t ${PLUGIN_NAME_ROOTFS} .

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
	@if [ "${TEST_TOKEN}" = "xyz" ] || [ "${TEST_LABEL}" = "xyz" ] ; then \
		echo -en "#############################\nYou must set TEST_* Variables\n#############################\n"; exit 1; fi

test-setup:
	@docker plugin set $(PLUGIN_NAME) linode-token=${TEST_TOKEN} linode-label=${TEST_LABEL}
	docker plugin enable $(PLUGIN_NAME)

check:
	docker run --rm -v $(PWD):/app -w /app golangci/golangci-lint:v1.46.2 golangci-lint run -v

unit-test:
	GOOS=linux go test

.PHONY clean:
	rm -fr $(PLUGIN_DIR)

clean-volumes:
	docker volume ls -q | grep 'test-' | xargs docker volume rm
clean-installed-plugins:
	docker plugin ls | grep linode | grep -v ID | awk '{print $$1}' | xargs docker plugin rm -f

