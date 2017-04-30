#!/bin/bash

set -euC

pkgname=arp242.net/trackwall

# Cache some stuff
go test -race -covermode=atomic -i .

find_deps() {
	(
		echo "$1"
		go list -f $'{{range $f := .Deps}}{{$f}}\n{{end}}' "$1"
		go list -f $'{{range $f := .TestImports}}{{$f}}\n{{end}}' "$1" | 
			while read testImp; do
				go list -f $'{{range $f := .Deps}}{{$f}}\n{{end}}' "$testImp";
			done
	) | sort -u | grep ^$pkgname | grep -v /vendor/ |
		tr '\n' ' ' | sed 's/ $//' | tr ' ' ','
}

echo >| coverage.txt
for pkg in $(go list ./... | grep -v /vendor/); do
	deps=$(find_deps "$pkg")
	go test -race \
		-covermode=atomic \
		-coverprofile=coverage.tmp \
		-coverpkg=$deps \
		"$pkg"
	if [ -f coverage.tmp ]; then
		cat coverage.tmp >> coverage.txt
		rm coverage.tmp
	fi
done

[ -n "${TRAVIS:-}" ] && curl -s https://codecov.io/bash | bash
rm coverage.txt
