#!/bin/bash

# set:
#   -e: Exit on error.
#   -x: Display commands.
set -ex

scriptDir=`dirname $(readlink -f $0)`
source $scriptDir/common.sh


# Deploy Step have to build again, it does not remember
# the docker image built before.
build


# Deploy Step
deploy

