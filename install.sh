#!/bin/sh

set -euC

# Just to be sure that no one actually runs this
echo "TODO"; exit 2

if [ $(id -u) != 0 ]; then
	echo "You need root priviliges for this"
	exit 1
fi

if [ $(uname) != "Linux" ] && [ $(uname) != "OpenBSD" ]; then
	echo "Sorry, only Linux and OpenBSD are supported. Follow the manual installation steps in the README."
	exit 2
fi

echo -n "User name (enter for _dnsblock): "
read user
[ -z "$user" ] && user=_dnsblock

echo -n "Chroot directory (enter for /var/run/dnsblock): "
read chroot
[ -z "$chroot" ] && chroot=/var/run/dnsblock

echo -n "IP to listen on (enter for 127.0.0.53): "
read listen
[ -z "$listen" ] && listen=127.0.0.53

sfwd=$(grep ^nameserver /etc/resolv.conf | cut -d ' ' -f2 | head -n1)
echo -n "Forward dns requests to (enter for $sfwd): "
read fwd
[ -z "$fwd" ] && fwd=$sfwd

if [ "$(uname)" = "Linux" ]; then
	# User
	if grep "^$user:"; then
		echo "User $user already exists, not creating it"
	else
		useradd "$user" -d "$chroot" -s /bin/false
	fi

	# resolv.conf
	# Debian
	if [ -f /etc/debian_version ]; then
		# TODO: is this enough?
		resolvconf --disable-updates
	else
		echo "IMPORTANT! It's not clear how to disable updates to your /etc/resolv.conf"
		echo "Unfortunately, there are many different network systems on Linux"
	fi
elif [ "$(uname)" = "OpenBSD" ]; then
	# User
	if grep "^$user:"; then
		echo "User $user already exists, not creating it"
	else
		# TODO
		#useradd "$user" -d "$chroot" -s /bin/false
	fi

	# TODO: set DNS server
fi

# Chroot
mkdir -p "$chroot"
chown "$user":"$user" "$chroot"

# root CA
openssl genrsa -out "$chroot/rootCA.key" 2048
openssl req -x509 -new -nodes -key "$chroot/rootCA.key" -sha256 -days 1024 -out "$chroot/rootCA.pem"
chown _dnsblock:_dnsblock "$chroot/rootCA"*
chmod 600 "$chroot/rootCA"*

# Resolv.conf
echo "nameserver $listen" >| /etc/resolv.conf

# TODO: Write out config from template
