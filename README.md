# Docker Volume Driver For Linode

[![GoDoc](https://godoc.org/github.com/libgolang/docker-volume-linode?status.svg)](https://godoc.org/github.com/libgolang/docker-volume-linode)
[![Go Report Card](https://goreportcard.com/badge/github.com/libgolang/docker-volume-linode)](https://goreportcard.com/report/github.com/libgolang/docker-volume-linode)
[![Build Status](https://travis-ci.org/libgolang/docker-volume-linode.svg?branch=master)](https://travis-ci.org/libgolang/docker-volume-linode)

## Requirements

- Linux (tested on Ubuntu 18.04, should work with other versions and distros)
- Docker (tested on version 17, should work with other versions)

## Installation

### Install

```sh
docker plugin install libgolang/docker-volume-linode
```

### Configuration

```sh
docker plugin set libgolang/docker-volume-linode LINODE_TOKEN=<linode token>
docker plugin set libgolang/docker-volume-linode LINODE_REGION=<linode region>
docker plugin set libgolang/docker-volume-linode LINODE_LABEL=<host label>
```

### Enable

```sh
docker plugin enable libgolang/docker-volume-linode
```

- Debugging Configuration

```sh
docker plugin set libgolang/docker-volume-linode LOG_LEVEL=debug
```


## Usage




### Create Volume

```sh
$ docker volume create -d linode-driver my-test-volume
my-test-volume
```

### Create 50G Volume

```sh
$ docker volume create -o size=50 -d linode-driver my-test-volume-50
my-test-volume-50
```

### List Volumes

```sh
$ docker volume ls
DRIVER              VOLUME NAME
linode-driver       my-test-volume
linode-driver       my-test-volume-50
```

### Remove Volumes

```sh
$ docker volume rm my-test-volume
my-test-volume

$ docker volume rm my-test-volume-50
my-test-volume-50
```

### Create and Use Linode Volume

```sh
$ docker volume create -d linode-driver http-volume
http-volume

$ docker run --rm -it -v http-volume:/usr/local/apache2/htdocs/ httpd
...
...
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

## Manual Installation

### Requirements

- Install Golang: <https://golang.org/>
- Get code and Compile: `go get -u github.com/libgolang/docker-volume-linode`

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

#### Enable Deug Level on plugin

```sh
docker plugin set libgolang/docker-volume-linode LOG_LEVEL=debug
```

#### Enable Deug Level in manual installation

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
