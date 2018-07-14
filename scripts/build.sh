#!/bin/bash

# set:
#   -e: Exit on error.
#   -x: Display commands.
set -ex

scriptDir=`dirname $(readlink -f $0)`
source $scriptDir/common.sh

# Build Step
build

