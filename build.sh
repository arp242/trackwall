#!/bin/sh

set -euC

root="$(dirname "$(readlink -f "$0")")"

set -x
export GOPATH="$root"

[ -d "$root/src/github.com/miekg/dns" ] || go get -u github.com/miekg/dns
[ -d "$root/src/github.com/davecgh/go-spew/spew" ] || go get -u github.com/davecgh/go-spew/spew
[ -d "$root/src/golang.org/x/sys/unix" ] || go get -u golang.org/x/sys/unix

go build -ldflags '-s -w' -x -v code.arp242.net/dnsblock
#go build -x -v dnsblock

set -x
echo "\nBuilt $root/dnsblock"
