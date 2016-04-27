Project status: experimental; it works for the author, and may be useful to
others, but may also sacrifice your firstborn to Cthulhu and take out a new
mortgage on your house.

-----------------------------------------

DNS proxy and filter.

The intended and most common usage is to block third-party browser requests.
It's not intended to "block advertisement" as such, rather it's intended to
"block third party requests".

It's inspired by adsuck, which unfortunately hasn't been updated in a few years
and is suffering from some problems.

Advantages:

- Lightweight.
- Browser independent. Also works for requests outside the browser.

To be fair, there are some disadvantages as well:

- More difficult to set up.
- Unable to filter by URL (only domain name). This is usually not a problem
  unless you want to filter out every last advertisement (which is not this
  program's goal).

Getting started
===============

Download and building
---------------------
dnsblock is written in Go, so you'll need that. Tested systems are:

- Go 1.5 on OpenBSD 5.9
- Go 1.6 on Ubuntu 16.04

Other POSIX systems should also work.

You can download and build it with:

	$ hg clone http://code.arp242.net/dnsblock
	$ cd dnsblock
	$ ./build.sh

Generic setup
-------------
The setup steps are somehow OS-specific, the `./instal.sh` script should take of
that though. It's interactive and asks for confirmation on every step. Here are
the general instructions:

- Add the `_dnsblock` user.

- Make the chroot directory.

- Make a X509 certificate in the chroot directory. This is required to serve
  surrogate scripts over https. dnsblock will generate a new certificate with
  the correct domain name signed with this root CA. You'll have to install it in
  your OS/browser to make this work.

  **Keep these files private!**

- You may also want to configure your browser (see "Browser setup" section).

Specific instructions for various systems are below (again, this is the same as
what `./install.sh` does).

OS specific setup
-----------------

### Ubuntu
- Setup user and chroot directory:

		$ useradd _dnsblock -d /var/run/dnsblock -s /bin/false
		$ mkdir /var/run/dnsblock
		$ chown _dnsblock:_dnsblock /var/run/dnsblock/

- Generate a root certificate:

		$ openssl genrsa -out /var/run/dnsblock/rootCA.key 2048
		$ openssl req -x509 -new -nodes -key /var/run/dnsblock/rootCA.key -sha256 -days 1024 -out /var/run/dnsblock/rootCA.pem

		$ chown _dnsblock:_dnsblock /var/run/dnsblock/rootCA*
		$ chmod 600 /var/run/dnsblock/rootCA*

- Setup resolv.conf:

		$ sudo resolvconf --disable-updates

	And then edit `/etc/resolv.conf` by hand. Needs to be done after every
	reboot.

	There is probably a better way of doing this, but fuck, this is an
	overcomplicated piece of shit with retarded documentation and at least three
	different "standard" ways of configuring the fucking network...

### OpenBSD
- Setup user and chroot directory:

		$ useradd -d /var/run/dnsblock -s /sbin/nologin _dnsblock
		$ mkdir /var/run/dnsblock
		$ chown _dnsblock:_dnsblock /var/run/dnsblock/

- Generate a root certificate:

		$ openssl genrsa -out /var/run/dnsblock/rootCA.key 2048
		$ openssl req -x509 -new -nodes -key /var/run/dnsblock/rootCA.key -sha256 -days 1024 -out /var/run/dnsblock/rootCA.pem

		$ chown _dnsblock:_dnsblock /var/run/dnsblock/rootCA*
		$ chmod 600 /var/run/dnsblock/rootCA*

- Setup resolv.conf:

	Set `/etc/dhclient.conf` to something like:

		# The installer adds this line, not strictly needed
		send host-name "yourhostname";

		# List our nameserver first
		prepend domain-name-servers 127.0.0.53;

	Running `sh /etc/netstart` will apply the settings.

- Set alias for `lo0`:

	If you want to listen on the loopback interface that is not `127.0.0.1` (which
	is the case by default) you'll have to add that address as an alias, which can
	be done in `/etc/hostname.lo0`:

		inet alias 127.0.0.53

	Running `sh /etc/netstart` will apply the settings.

Browser setup
--------------
TODO

- DNS cache?


How it works
============
An address like:

	http://blocked.example.com/some_script.js

will be resolved to:

	http://127.0.0.53:80/some_script.js

dnsblock also runs a HTTP server at `127.0.0.53:80` which will:

- Serve some no-ops for some common scripts so webpages don't error out (Google
  Analytics, AddThis).
- Serve a simply 0-byte response with a guessed Content-Type

ChangeLog
=========
No release yet. This is experimental software.

TODO
====
- Figure out a better name
- Write proper installation instructions (and probably script as well)
- Better https
- Listen to signals to reload
- Measure some degree of performance

FAQ
===

Will this serve as a local DNS resolver and/or cache?
-----------------------------------------------------
No. This is not a DNS resolver/cache, just a proxy/filter. If you're looking for
a DNS cache, then [unbound][unbound] is a good option. Both dnsblock and unbound
take up about 10M of memory at the most (in the case of dnsblock this depends on
the number of hosts to block) so running both on even an older system should be
fine.

This program sucks. What alternatives are there?
================================================
Sorry :-( I always appreciate feedback by the way, so drop me a email at
martin@arp242.net

Here are some similar programs:

- [adsuck](https://github.com/conformal/adsuck) (POSIX systems)
- [Little Snitch](https://www.obdev.at/products/littlesnitch/index.html) (OSX)


Authors and license
===================
This program is written by Martin Tournoij <martin@arp242.net>, whose job would
have been a lot harder without [Miek Gieben's DNS library][dns].

The MIT License (MIT)

Copyright © 2016 Martin Tournoij

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to
deal in the Software without restriction, including without limitation the
rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

The software is provided "as is", without warranty of any kind, express or
implied, including but not limited to the warranties of merchantability,
fitness for a particular purpose and noninfringement. In no event shall the
authors or copyright holders be liable for any claim, damages or other
liability, whether in an action of contract, tort or otherwise, arising
from, out of or in connection with the software or the use or other dealings
in the software.

[dns]: https://godoc.org/github.com/miekg/dns
[unbound]: https://unbound.net/
