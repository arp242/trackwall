#!/bin/sh

set -euC

root="$(dirname "$(readlink -f "$0")")"

set -x
export GOPATH="$root"

go test trackwall
