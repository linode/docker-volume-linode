#!/bin/bash

# set:
#   -e: Exit on error.
#   -x: Display commands.
set -ex

# Tools
go get -u github.com/tsenart/deadcode
go get -u github.com/kisielk/errcheck
go get -u golang.org/x/lint/golint

# Run Code Tests
go vet
errcheck
golint
deadcode
go test

# Test Plugin Functionality
# ...
# ...
