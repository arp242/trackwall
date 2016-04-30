#!/bin/sh

set -euC

cp -vf dnsblock /usr/local/bin/
mkdir -pv /etc/dnsblock
cp -v config* /etc/dnsblock/

uname=$(uname)
lsb=$(lsb_release -is || true)

# runit
if [ $lsb = "VoidLinux" ]; then
	mkdir -vp /etc/sv/dnsblock/log /var/log/dnsblock
	chown -v _dnsblock:_dnsblock /var/log/dnsblock

	cp -v ./sv/runit /etc/sv/dnsblock/run
	chmod -v a+x /etc/sv/dnsblock/run

	cp -v ./sv/runit.log /etc/sv/dnsblock/log/run
	chmod -v a+x /etc/sv/dnsblock/log/run

	ln -fvs /etc/sv/dnsblock /var/service/
else
	echo "Unsupported OS or Linux flavour."
fi
