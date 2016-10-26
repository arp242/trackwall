#!/bin/sh

set -euC

root="$(dirname "$(readlink -f "$0")")"

export GO15VENDOREXPERIMENT=1
export GOPATH="$root"

# -race doesn't work on all platforms
go build -race arp242.net/trackwall || go build arp242.net/trackwall
