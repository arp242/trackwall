#!/bin/sh

set -euC

prefix="/usr/local"
etcdir="/etc"
user=_dnsblock
name=dnsblock

echo "Installing $prefix/sbin/$name"
out=dnsblock-$(uname -sm | tr '[[:upper:]] ' '[[:lower:]]-')
install "$out" "$prefix/sbin/$name"

[ -e "$etcdir/$name" ] || mkdir -pv "$etcdir/$name"
for f in config*; do
	if [ ! -e "$etcdir/$name/$f" ]; then
		echo "Installing $etcdir/$name/$f"
		install -m 0644 "$f" "$etcdir/$name/$f"
	fi
done

uname=$(uname)

unsup() {
	echo $1
	exit 1
}

if [ "$uname" = "OpenBSD" ]; then
	if ! grep "^$user" /etc/passwd; then
		echo "Adding user $user"
		useradd -d "/var/dnsblock" -s /sbin/nologin "$user"
	fi

	echo "installing /etc/rc.d/$name"
	install init/openbsd "/etc/rc.d/$name"
elif [ "$uname" = "Linux" ]; then
	lsb=$(lsb_release -is || true)

	# runit
	if [ "$lsb" = "VoidLinux" ]; then
		if ! grep "^$user" /etc/passwd; then
			echo "Adding user $user"
			useradd "$user" -d /var/dnsblock -s /sbin/nologin
		fi

		mkdir -vp "$etcdir/sv/dnsblock/log" /var/log/dnsblock
		chown -v "$user":"$user" /var/log/dnsblock

		cp -v ./init/runit "$etcdir/sv/dnsblock/run"
		chmod -v a+x "$etcdir/sv/dnsblock/run"
		cp -v ./init/runit.log "$etcdir/sv/dnsblock/log/run"
		chmod -v a+x "$etcdir/sv/dnsblock/log/run"

		ln -fvs "$etcdir/sv/dnsblock" /var/service/
	elif [ "$lsb" = "Ubuntu" ] || [ "$lsb" = "Debian" ] ; then
		if [ ! -x /bin/systemctl ]; then
			unsup "Currently only Debian/Ubuntu with systemd is supported."
		fi

		cp -v ./init/systemd.service "/etc/systemd/system/$name.service"
		systemctl daemon-reload
	else
		unsup "Unsupported Linux flavour; no init files installed (perhaps one of the files in ./init/ will work though?)"
	fi
else
	unsup "Unsupported OS; no init files installed (perhaps one of the files in ./init/ will work though?)"
	exit 1
fi
