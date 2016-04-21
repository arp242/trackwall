#!/bin/sh

root="$(dirname "$(readlink -f "$0")")"
export GOPATH="$root"
go run "$root/src/dnsblock/main.go" $@
