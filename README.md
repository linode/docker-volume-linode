# Docker Volume Driver For Linode

[![GoDoc](https://godoc.org/github.com/libgolang/docker-volume-linode?status.svg)](https://godoc.org/github.com/libgolang/docker-volume-linode)
[![Go Report Card](https://goreportcard.com/badge/github.com/libgolang/docker-volume-linode)](https://goreportcard.com/report/github.com/libgolang/docker-volume-linode)
[![Build Status](https://travis-ci.org/libgolang/docker-volume-linode.svg?branch=master)](https://travis-ci.org/libgolang/docker-volume-linode)

## Requirements
- Linux (tested on Ubuntu 18.04, should work with other versions)
- Docker (tested on version 17, should work with other versions)

## Usage

### Run the driver
```
export LINODE_TOKEN=<token from linode console>
export LINODE_REGION=us-west
export LINODE_LABEL=linode-machine-label-1234
./docker-volume-linode 
```
or

```
./docker-volume-linode -linode.token $TOKEN -linode.region $REGION -linode.host $LINODE_LABEL
```


### Create Volume
```
$ docker volume create -d linode-driver my-test-volume
| my-test-volume
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
