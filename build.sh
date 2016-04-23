#!/bin/sh

set -euC

root="$(dirname "$(readlink -f "$0")")"
export GOPATH="$root"
#go build -ldflags '-s -w' -x -v dnsblock
go build -x -v dnsblock

echo
echo "Built $root/dnsblock"
