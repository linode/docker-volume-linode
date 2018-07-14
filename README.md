# Docker Volume Driver For Linode

[![GoDoc](https://godoc.org/github.com/libgolang/docker-volume-linode?status.svg)](https://godoc.org/github.com/libgolang/docker-volume-linode)
[![Go Report Card](https://goreportcard.com/badge/github.com/libgolang/docker-volume-linode)](https://goreportcard.com/report/github.com/libgolang/docker-volume-linode)
[![Build Status](https://travis-ci.org/libgolang/docker-volume-linode.svg?branch=master)](https://travis-ci.org/libgolang/docker-volume-linode)

## Requirements
- Linux (tested on Ubuntu 18.04, should work with other versions and distros)
- Docker (tested on version 17, should work with other versions)

## Installation

- Install

  ```
  docker plugin install libgolang/docker-volume-linode
  ```

- Configuration

  ```
  docker plugin set libgolang/docker-volume-linode LINODE_TOKEN=<linode token>
  docker plugin set libgolang/docker-volume-linode LINODE_REGION=<linode region>
  docker plugin set libgolang/docker-volume-linode LINODE_LABEL=<host label>
  ```

- Enable

  ```
  docker plugin enable libgolang/docker-volume-linode
  ```


- Debugging Configuration

  ```
  docker plugin set libgolang/docker-volume-linode LOG_LEVEL=debug
  ```


## Usage

### Create Volume

```
$ docker volume create -d linode-driver my-test-volume
```

  ```
  my-test-volume
  ```

### Create 50G Volume
```
$ # docker volume create -o size=50 -d linode-driver my-test-volume-50
| my-test-volume-50
```

### List Volumes

```
$ docker volume ls
| DRIVER              VOLUME NAME
| linode-driver       my-test-volume
| linode-driver       my-test-volume-50
```

### Remove Volumes
```
$ docker volume rm my-test-volume
| my-test-volume
$ docker volume rm my-test-volume-50
| my-test-volume-50
```


### Create and Use Linode Volume
```
$ docker volume create -d linode-driver http-volume
| http-volume
$ docker run --rm -it -v http-volume:/usr/local/apache2/htdocs/ httpd
| ...
| ...
```



# Manual Installation

# Requirements
- Install Golang: https://golang.org/
- Get code and Compile: `go get -u github.com/libgolang/docker-volume-linode`

### Run the driver

```
$ docker-volume-linode --linode-token=<token from linode console> --linode-region=<linode region> --linode-label=<host label>
```
or

```
$ export LINODE_TOKEN=<token from linode console>
$ export LINODE_REGION=<linode region>
$ export LINODE_LABEL=<host label>
$ docker-volume-linode
```



## Debugging

# Enable Deug Level on plugin

  ```
  docker plugin set libgolang/docker-volume-linode LOG_LEVEL=debug
  ```

# Enable Deug Level in manual installation
```
$ docker-volume-linode --linode-token=<...> --linode-region=<...> --linode-label=<...> --log-level=debug
```
or

```
$ export DEBUG_LEVEL=debug
$ export LINODE_REGION=<...>
$ export LINODE_LABEL=<...>
$ export LINODE_LABEL=<...>
$ docker-volume-linode
```


## Tested On
```
Ubuntu 18.04 LTS
```

```
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
