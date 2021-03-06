#!/usr/bin/env bash
#
# Lint all modules with golangci-lint.
#
#   lint.sh - lints all modules in the repository root, or the current folder if
#   not in a git workspace.
#
#   lint.sh [folder] - lints all modules in the specified folder.

REPO=`git rev-parse --show-toplevel 2> /dev/null`
if [[ $? != 0 ]]; then
    REPO=`pwd`
fi

# Root is the input argument, or REPO if no input argument.
ROOT=${1:-$REPO}
echo "Linting all modules under" $ROOT

set -e

shopt -s expand_aliases

export GO111MODULE=on

go version

pushd $ROOT > /dev/null

MODPATHS=$(find . -name go.mod -type f -print)

alias superlint='golangci-lint run --deadline=10m \
    --disable-all \
    --enable govet \
    --enable staticcheck \
    --enable gosimple \
    --enable unconvert \
    --enable ineffassign \
    --enable structcheck \
    --enable goimports \
    --enable misspell \
    --enable unparam \
    --enable asciicheck \
    --enable makezero'

# run lint on all listed modules
set +e
ERROR=0
trap 'ERROR=$?' ERR
for MODPATH in $MODPATHS; do
    module=$(dirname ${MODPATH})
    pushd $module > /dev/null
    MOD=$(go list -m)
    echo "Linting:" $MOD
    superlint
    popd > /dev/null
done

popd > /dev/null # ROOT

if [ $ERROR -ne 0 ]; then
    exit $ERROR
fi
