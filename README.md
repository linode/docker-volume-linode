# Docker Volume Driver For Linode

[![GoDoc](https://godoc.org/github.com/linode/docker-volume-linode?status.svg)](https://godoc.org/github.com/linode/docker-volume-linode)
[![Go Report Card](https://goreportcard.com/badge/github.com/linode/docker-volume-linode)](https://goreportcard.com/report/github.com/linode/docker-volume-linode)
[![Build Status](https://travis-ci.org/linode/docker-volume-linode.svg?branch=master)](https://travis-ci.org/linode/docker-volume-linode)

## Requirements

- Linux (tested on Ubuntu 18.04, should work with other versions and distros)
- Docker (tested on version 17, should work with other versions)

## Installation

### Install

```sh
docker plugin install linode/docker-volume-linode
```

### Configuration

```sh
docker plugin set linode/docker-volume-linode LINODE_TOKEN=<linode token>
docker plugin set linode/docker-volume-linode LINODE_REGION=<linode region>
docker plugin set linode/docker-volume-linode LINODE_LABEL=<host label>
```

List or regions can be found at:  https://api.linode.com/v4/regions


### Enable

```sh
docker plugin enable linode/docker-volume-linode
```

- Debugging Configuration

```sh
docker plugin set linode/docker-volume-linode LOG_LEVEL=debug
```


## Usage




### Create Volume

```sh
$ docker volume create -d linode/docker-volume-linode my-test-volume
my-test-volume
```

### Create 50G Volume

```sh
$ docker volume create -o size=50 -d linode/docker-volume-linode my-test-volume-50
my-test-volume-50
```

### List Volumes

```sh
$ docker volume ls
DRIVER                               VOLUME NAME
linode/docker-volume-linode       my-test-volume
linode/docker-volume-linode       my-test-volume-50
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
| linode-label | **Required** The Linode Label to attach block storage volumes to (defaults to the system hostname) |
| linode-region | The Linode region to create volumes in (inferred if using linode-label, defaults to us-west) |
| socket-file | Sets the socket file/address (defaults to /run/docker/plugins/linode-driver.sock) |
| socket-gid | Sets the socket GID (defaults to 0) |
| mount-root | Sets the root directory for volume mounts (default /mnt) |
| log-level | Log Level (defaults to WARN) |
| log-trace | Set Tracing to true (defaults to false) |

Options can be set once for all future uses with [`docker plugin set`](https://docs.docker.com/engine/reference/commandline/plugin_set/#extended-description).

## Manual Installation

### Requirements

- Install Golang: <https://golang.org/>
- Get code and Compile: `go get -u github.com/linode/docker-volume-linode`

### Run the driver

```sh
docker-volume-linode --linode-token=<token from linode console> --linode-region=<linode region> --linode-label=<linode label>
```

or

```sh
export LINODE_TOKEN=<token from linode console>
export LINODE_REGION=<linode region>
export LINODE_LABEL=<linode label>
docker-volume-linode
```

### Debugging

#### Enable Debug Level on plugin

The driver name when running manually is the same name as the socket file.

```sh
docker plugin set docker-volume-linode LOG_LEVEL=debug
```

#### Enable Debug Level in manual installation

```sh
docker-volume-linode --linode-token=<...> --linode-region=<...> --linode-label=<...> --log-level=debug
```

or

```sh
export DEBUG_LEVEL=debug
export LINODE_REGION=<...>
export LINODE_LABEL=<...>
export LINODE_LABEL=<...>
docker-volume-linode
```

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
