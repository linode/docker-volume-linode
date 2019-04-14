# Docker Volume Driver For Linode

[![GoDoc](https://godoc.org/github.com/linode/docker-volume-linode?status.svg)](https://godoc.org/github.com/linode/docker-volume-linode)
[![Go Report Card](https://goreportcard.com/badge/github.com/linode/docker-volume-linode)](https://goreportcard.com/report/github.com/linode/docker-volume-linode)
[![Build Status](https://travis-ci.org/linode/docker-volume-linode.svg?branch=master)](https://travis-ci.org/linode/docker-volume-linode)

This [volume plugin](https://docs.docker.com/engine/extend/plugins_volume/) adds the ability to manage [Linode Block Storage](https://www.linode.com/blockstorage) as [Docker Volumes](https://docs.docker.com/storage/volumes/).
[Good use cases for volumes](https://docs.docker.com/storage/#good-use-cases-for-volumes) include off-node storage to avoid size constraints or moving a container and the related volume between nodes in a [Swarm](https://github.com/linode/docker-machine-driver-linode#provisioning-docker-swarm).

## Requirements

- Linux (tested on Ubuntu 18.04, should work with other versions and distributions)
- Docker (tested on version 17, should work with other versions)

## Installation

When the system hostname is the Linode label, the only required parameter is the `linode-token`:

```sh
docker plugin install --alias linode --grant-all-permissions linode/docker-volume-linode linode-token=<linode token>
```

### Changing the plugin configuration

The plugin can also be configured (or reconfigured) in multiple steps.

```sh
docker plugin install --alias linode linode/docker-volume-linode
docker plugin disable linode
docker plugin set linode linode-token=<linode token>
docker plugin set linode linode-label=<linode label>
docker plugin enable linode
```

- \<linode token\>: You will need a Linode APIv4 Personal Access Token.  Get one here: <https://developers.linode.com/api/v4#section/Personal-Access-Token>.  The API Token must have Read/Write permission for Volumes and Linodes.
- \<linode label\>: The label given to the host Linode Control Panel. Defaults to the system hostname.
  [Some Linode regions do not have Block Storage Volume support](https://www.linode.com/community/questions/344/when-will-block-storage-be-available-in-my-datacenter), such as: `us-southeast` and `ap-northeast-1a`.  For a complete list of regions:  https://api.linode.com/v4/regions
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

### Driver Options

| Option Name | Description |
| --- | --- |
| linode-token | **Required** The Linode APIv4 [Personal Access Token](https://cloud.linode.com/profile/tokens)
| linode-label | The Linode Label to attach block storage volumes to (defaults to the system hostname) |
| socket-file | Sets the socket file/address (defaults to /run/docker/plugins/linode.sock) |
| socket-gid | Sets the socket GID (defaults to 0) |
| mount-root | Sets the root directory for volume mounts (default /mnt) |
| log-level | Sets log level to debug,info,warn,error (defaults to info) |

Options can be set once for all future uses with [`docker plugin set`](https://docs.docker.com/engine/reference/commandline/plugin_set/#extended-description).

## Manual Installation

- Install Golang: <https://golang.org/>
- Get code and Compile: `go get -u github.com/linode/docker-volume-linode`

### Run the driver

```sh
docker-volume-linode --linode-token=<token from linode console> --linode-label=<linode label>
```

### Debugging

#### Enable Debug Level on plugin

The driver name when running manually is the same name as the socket file.

```sh
docker plugin set docker-volume-linode LOG_LEVEL=debug
```

#### Enable Debug Level in manual installation

```sh
docker-volume-linode --linode-token=<...> --linode-label=<...> --log-level=debug
```

## Development

A great place to get started is the [Docker Engine managed plugin system] documentation](https://docs.docker.com/engine/extend/#create-a-volumedriver).

## Tested On

```text
Ubuntu 18.04 LTS
```

```text
Tested With:
Client:
 Version:       17.12.1-ce
 API version:   1.35
 Go version:    go1.10.1
 Git commit:    7390fc6
 Built: Wed Apr 18 01:23:11 2018
 OS/Arch:       linux/amd64

Server:
 Engine:
  Version:      17.12.1-ce
  API version:  1.35 (minimum version 1.12)
  Go version:   go1.10.1
  Git commit:   7390fc6
  Built:        Wed Feb 28 17:46:05 2018
  OS/Arch:      linux/amd64
  Experimental: false
```

## Discussion / Help

Join us at [#linodego](https://gophers.slack.com/messages/CAG93EB2S) on the [gophers slack](https://gophers.slack.com)
