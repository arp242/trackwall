#!/bin/sh

set -euC

root="$(dirname "$(readlink -f "$0")")"

export GO15VENDOREXPERIMENT=1
export GOPATH="$root"

go build code.arp242.net/dnsblock
