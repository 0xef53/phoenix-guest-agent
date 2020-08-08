#!/bin/bash
set -eu

declare -r GOOS="linux"
declare -r GOARCH="amd64"

declare -r USER_GROUP="$(stat --printf "%u:%g" ./cmd)"
trap "chown -R $USER_GROUP ./bin" 0

go version
go fmt "./..."

go install -ldflags "-s -w" "./cmd/..."

exit 0
