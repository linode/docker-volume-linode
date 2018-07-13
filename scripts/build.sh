#!/bin/bash

set -e

scriptDir=`dirname $(readlink -f $0)`
source $scriptDir/common.sh

##############################
echo "go get -u github.com/golang/dep/cmd/dep"
go get -u github.com/golang/dep/cmd/dep
##############################
echo "dep ensure"
dep ensure
##############################
echo "docker build --no-cache -q -t ${PLUGIN_NAME_ROOTFS} ."
docker build --no-cache -q -t ${PLUGIN_NAME_ROOTFS} .
##############################
echo "mkdir -p ./plugin/rootfs"
mkdir -p ./plugin/rootfs
##############################
echo "docker create --name tmp  ${PLUGIN_NAME_ROOTFS}"
docker create --name tmp  ${PLUGIN_NAME_ROOTFS}
##############################
echo "docker export tmp | tar -x -C ./plugin/rootfs"
docker export tmp | tar -x -C ./plugin/rootfs
##############################
echo "cp config.json ./plugin/"
cp config.json ./plugin/
##############################
echo "docker rm -vf tmp"
docker rm -vf tmp
##############################
echo "docker plugin rm -f ${PLUGIN_NAME} || true"
docker plugin rm -f ${PLUGIN_NAME} || true
##############################
echo "docker plugin create ${PLUGIN_NAME} ./plugin"
docker plugin create ${PLUGIN_NAME} ./plugin
##############################
