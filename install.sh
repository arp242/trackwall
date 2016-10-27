#!/bin/sh

set -euC

if [ $(id -u) -eq 0 ]; then
	echo "Don't run this as root; we will ask for root permission with sudo when needed."
	exit 1
fi

uname=$(uname)

if [ "$uname" = "OpenBSD" ]; then
	sudo=doas
else
	# TODO: Fall back to su if sudo doesn't exist
	sudo=sudo
fi

prefix=/usr/local
etcdir=/etc
user=_trackwall
name=trackwall

#go get -u arp242.net/trackwall
go install arp242.net/trackwall

echo "Installing $prefix/sbin/$name"
# TODO: re-exec after go install with sudo
$sudo install "$GOPATH/bin/$name" "$prefix/sbin/$name"

[ -e "$etcdir/$name" ] || mkdir -pv "$etcdir/$name"
for f in config*; do
	if [ ! -e "$etcdir/$name/$f" ]; then
		echo "Installing $etcdir/$name/$f"
		sudo install -m 0644 "$f" "$etcdir/$name/$f"
	fi
done

unsup() {
	echo $1
	exit 1
}

init_runit() {
	mkdir -vp "$etcdir/sv/trackwall/log" /var/log/trackwall
	chown -v "$user":"$user" /var/log/trackwall

	cp -v ./init/runit "$etcdir/sv/trackwall/run"
	chmod -v a+x "$etcdir/sv/trackwall/run"
	cp -v ./init/runit.log "$etcdir/sv/trackwall/log/run"
	chmod -v a+x "$etcdir/sv/trackwall/log/run"

	ln -fvs "$etcdir/sv/trackwall" /var/service/
}

init_systemd() {
	$sudo cp -v ./init/systemd.service "/etc/systemd/system/$name.service"
	$sudo systemctl daemon-reload
}


if [ "$uname" = "OpenBSD" ]; then
	if ! grep "^$user" /etc/passwd; then
		echo "Adding user $user"
		useradd -d "/var/trackwall" -s /sbin/nologin "$user"
	fi

	echo "installing /etc/rc.d/$name"
	install init/openbsd "/etc/rc.d/$name"
elif [ "$uname" = "Linux" ]; then
	lsb=$(lsb_release -is 2>&1 || true)

	if ! grep -q "^$user" /etc/passwd; then
		echo "Adding user $user"
		useradd "$user" -d /var/trackwall -s /sbin/nologin
	fi

	if [ "$lsb" = "VoidLinux" ]; then
		init=runit
	elif [ "$lsb" = "Ubuntu" ] || [ "$lsb" = "Debian" ] ; then
		if [ ! -x /bin/systemctl ]; then
			unsup "Currently only Debian/Ubuntu with systemd is supported."
		fi
		init=systemd
	elif [ -x /bin/systemctl ]; then
		init=systemd
	else
		unsup "Unsupported Linux flavour; no init files installed (perhaps one of the files in ./init/ will work though?)"
	fi

	init_$init
else
	unsup "Unsupported OS; no init files installed (perhaps one of the files in ./init/ will work though?)"
	exit 1
fi
