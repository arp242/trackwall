#!/bin/sh

set -euC

root="$(dirname "$(readlink -f "$0")")"

export GO15VENDOREXPERIMENT=1
export GOPATH="$root"

# -race doesn't work on all platforms
go build -race bitbucket.org/Carpetsmoker/dnsblock || go build bitbucket.org/Carpetsmoker/dnsblock
