#!/usr/bin/env bash

# usage:
# ./run_tests.sh                         # local, go 1.11
# ./run_tests.sh docker                  # docker, go 1.11
# ./run_tests.sh podman                  # podman, go 1.11

set -ex

# The script does automatic checking on a Go package and its sub-packages,
# including:
# 1. gofmt         (http://golang.org/cmd/gofmt/)
# 2. go vet        (http://golang.org/cmd/vet)
# 3. gosimple      (https://github.com/dominikh/go-simple)
# 4. unconvert     (https://github.com/mdempsky/unconvert)
# 5. ineffassign   (https://github.com/gordonklaus/ineffassign)
# 6. race detector (http://blog.golang.org/race-detector)

# golangci-lint (github.com/golangci/golangci-lint) is used to run each each
# static checker.

# Default GOVERSION
[[ ! "$GOVERSION" ]] && GOVERSION=1.11
REPO=dcrdata
ROOTMODULE="github.com/decred/dcrdata"
ALLMODULES="$ROOTMODULE/api/types
  $ROOTMODULE/blockdata
  $ROOTMODULE/db/cache
  $ROOTMODULE/db/dbtypes
  $ROOTMODULE/db/dcrpg
  $ROOTMODULE/db/dcrsqlite
  $ROOTMODULE/exchanges
  $ROOTMODULE/explorer/types
  $ROOTMODULE/gov/agendas
  $ROOTMODULE/gov/politeia
  $ROOTMODULE/mempool
  $ROOTMODULE/middleware
  $ROOTMODULE/pubsub
  $ROOTMODULE/pubsub/types
  $ROOTMODULE/rpcutils
  $ROOTMODULE/semver
  $ROOTMODULE/stakedb
  $ROOTMODULE/txhelpers"

testrepo () {
  TMPDIR=$(mktemp -d)
  TMPFILE=$(mktemp)
  export GO111MODULE=on

  go version

  # Test application install
  go build
  (cd cmd/rebuilddb && go build)
  (cd cmd/rebuilddb2 && go build)
  (cd cmd/scanblocks && go build)

  # Check tests
  git clone https://github.com/dcrlabs/bug-free-happiness $TMPDIR/test-data-repo
  tar xvf $TMPDIR/test-data-repo/stakedb/test_ticket_pool.bdgr.tar.xz -C ./stakedb

  env GORACE='halt_on_error=1' go test -v -race ./... $ALLMODULES

  # check linters
  golangci-lint run --deadline=10m --disable-all --enable govet --enable staticcheck \
    --enable gosimple --enable unconvert --enable ineffassign --enable structcheck \
    --enable goimports --enable misspell --enable unparam


  # webpack
  npm install
  npm run build

  echo "------------------------------------------"
  echo "Tests completed successfully!"

  # Remove all the tests data
  rm -rf $TMPDIR $TMPFILE
}

DOCKER=
[[ "$1" == "docker" || "$1" == "podman" ]] && DOCKER=$1
if [ ! "$DOCKER" ]; then
    testrepo
    exit
fi

DOCKER_IMAGE_TAG=dcrdata-golang-builder-$GOVERSION
$DOCKER pull decred/$DOCKER_IMAGE_TAG

$DOCKER run --rm -it -v $(pwd):/src decred/$DOCKER_IMAGE_TAG /bin/bash -c "\
  rsync -ra --include-from=<(git --git-dir=/src/.git ls-files) \
  --filter=':- .gitignore' \
  /src/ /go/src/github.com/decred/$REPO/ && \
  cd github.com/decred/$REPO/ && \
  env GOVERSION=$GOVERSION GO111MODULE=on bash run_tests.sh"
