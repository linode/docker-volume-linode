# Docker Volume Driver For Linode

[![Go Reference](https://pkg.go.dev/badge/github.com/linode/docker-volume-linode/linode.svg)](https://pkg.go.dev/github.com/linode/docker-volume-linode/)
[![Build](/../../actions/workflows/pull_request.yml/badge.svg)](/../../actions/workflows/pull_request.yml)

This [volume plugin](https://docs.docker.com/engine/extend/plugins_volume/) adds the ability to manage [Linode Block Storage](https://www.linode.com/blockstorage) as [Docker Volumes](https://docs.docker.com/storage/volumes/) from within a Linode.
[Good use cases for volumes](https://docs.docker.com/storage/#good-use-cases-for-volumes) include off-node storage to avoid size constraints or moving a container and the related volume between nodes in a [Swarm](https://github.com/linode/docker-machine-driver-linode#provisioning-docker-swarm).

## Requirements

- Linux (tested on Fedora 34, should work with other versions and distributions)
- Docker (tested on version 20, should work with other versions)

## Installation

```sh
docker plugin install --alias linode --grant-all-permissions \
linode/docker-volume-linode \
linode-token=<linode token>
```

### Driver Options

| Option Name | Description |
| --- | --- |
| linode-token | **Required** The Linode APIv4 [Personal Access Token](https://cloud.linode.com/profile/tokens) to use. (requires `linodes:read_write volumes:read_write events:read_only`)
| linode-label | The label of the current Linode. This is only necessary if your Linode does not have a resolvable Link Local IPv6 Address.
| force-attach | If true, volumes will be forcibly attached to the current Linode if already attached to another Linode. (defaults to false) WARNING: Forcibly reattaching volumes can result in data loss if a volume is not properly unmounted.
| mount-root | Sets the root directory for volume mounts (defaults to /mnt) |
| log-level | Sets log level to debug,info,warn,error (defaults to info) |
| socket-user | Sets the user to create the docker socket with (defaults to root) |

Options can be set once for all future uses with [`docker plugin set`](https://docs.docker.com/engine/reference/commandline/plugin_set/#extended-description).

### Changing the plugin configuration

The plugin can also be configured (or reconfigured) in multiple steps.

```sh
docker plugin install --alias linode linode/docker-volume-linode
docker plugin disable linode
docker plugin set linode linode-token=<linode token>
docker plugin enable linode
```

- For all options see [Driver Options](#Driver-Options) section

### Docker Swarm

Volumes can be mounted to one container at the time because Linux Block Storage volumes can only be attached to one Linode at the time.

## Usage

All examples assume the driver has been aliased to `linode`.

### Create Volume

Linode Block Storage volumes can be created and managed using the [docker volume create](https://docs.docker.com/engine/reference/commandline/volume_create/) command.

```sh
$ docker volume create -d linode my-test-volume
my-test-volume
```

If a named volume already exists on the Linode account and it is in the same region of the Linode, it will be reattached if possible.  A Linode Volume can be attached to a single Linode at a time.

#### Create Options

The driver offers [driver specific volume create options](https://docs.docker.com/engine/reference/commandline/volume_create/#driver-specific-options):

| Option | Type | Default | Description |
| ---    | ---  | ---     | ---         |
| `size` | int  | `10`    | the size (in GB) of the volume to be created.  Volumes must be at least 10GB in size, so the default is 10GB.
| `filesystem` | string | `ext4` | the filesystem argument for `mkfs` when formating the new (raw) volume (xfs, btrfs, ext4)
| `delete-on-remove` | bool | `false`| if the Linode volume should be deleted when removed

```sh
$ docker volume create -o size=50 -d linode my-test-volume-50
my-test-volume-50
```

Volumes can also be created and attached from `docker run`:

```sh
docker run -it --rm --mount volume-driver=linode,source=test-vol,destination=/test,volume-opt=size=25 alpine
```

Multiple create options can be supplied:

```sh
docker run -it --rm --mount volume-driver=linode,source=test-vol,destination=/test,volume-opt=size=25,volume-opt=filesystem=btrfs,volume-opt=delete-on-remove=true alpine
```

### List Volumes

```sh
$ docker volume ls
DRIVER              VOLUME NAME
linode:latest       my-test-volume
linode:latest       my-test-volume-50
```

### Use Volume

```sh
$ docker run --rm -it -v my-test-volume:/usr/local/apache2/htdocs/ httpd
...
```

### Remove Volumes

```sh
$ docker volume rm my-test-volume
my-test-volume

$ docker volume rm my-test-volume-50
my-test-volume-50
```

## Manual Installation

- Install Golang: <https://golang.org/>
- Get code and Compile: `go get -u github.com/linode/docker-volume-linode`

### Run the driver

```sh
docker-volume-linode --linode-token=<token from linode console>
```

### Debugging

#### Enable Debug Level on plugin

The driver name when running manually is the same name as the socket file.

```sh
docker plugin set docker-volume-linode log-level=debug
```

#### Enable Debug Level in manual installation

```sh
docker-volume-linode --linode-token=<...> --log-level=debug
```

## Development

A great place to get started is the Docker Engine managed plugin system [documentation](https://docs.docker.com/engine/extend/#create-a-volumedriver).

## Running Integration Tests

The integration tests for this project can be easily run using the `make quick-test` target.
This target provisions and connects to a Linode instance, uploads the plugin, builds it, enables it, 
and runs the integration test suite. Subsequent runs of this target will re-use the existing Linode instance.

In order to run this target, Ansible and the [Linode Ansible Collection](https://github.com/linode/ansible_linode/)
must be installed on the local machine:

```bash
pip install ansible

ansible-galaxy collection install linode.cloud

pip install -r https://raw.githubusercontent.com/linode/ansible_linode/main/requirements.txt
```

The integration test suite also requires that a full-access [Linode Personal Access Token](https://www.linode.com/docs/products/tools/api/guides/manage-api-tokens/)
be exported as the `LINODE_TOKEN` environment variable.

```bash
export LINODE_TOKEN=EXAMPLETOKEN
```

The integration test suite can now be run:

```bash
make quick-test
```

NOTE: This target requires an existing SSH key be created. If an SSH key exists at a path other than
`~/.ssh/id_rsa`, the `QUICKTEST_SSH_PUBKEY` argument can be specified:

```bash
make QUICKTEST_SSH_PUBKEY="~/.ssh/mykey.pub" quick-test
```

If you would like to create a test environment for docker-volume-linode without running the integration test suite, 
the `QUICKTEST_SKIP_TESTS` argument can be specified:

```bash
make QUICKTEST_SKIP_TESTS=1 quick-test
```

## Discussion / Help

Join us at [#linodego](https://gophers.slack.com/messages/CAG93EB2S) on the [gophers slack](https://gophers.slack.com)
