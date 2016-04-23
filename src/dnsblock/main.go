// DNS proxy which can spoof responses to block ads and malicious websites.
//
// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"
	"time"
)

const (
	RESPONSE_FORWARD  = 1
	RESPONSE_SPOOF    = 2
	RESPONSE_NXDOMAIN = 3
)

// Cache entry
type cache_t struct {
	// We don't cache the actual DNS responses − that's the resolver's job. We
	// just cache the action taken. That's enough and saves some time in
	// processing regexps and such
	// See the RESPONSE_* constants for the possible values.
	response uint8
	expires  int64
}

var (
	_config config_t

	// Static hosts added with hostlist/host. The key is the hostname, the
	// (optional) value is a surrogate script to serve.
	_hosts = make(map[string]string)

	// Compiled regexes added with regexlist/regex. Pre-compiling the surrogate
	// scripts isn't possible here.
	_regexps []*regexp.Regexp

	// Hosts to override; value is timestamp, once that's expired the entry will be
	// removed from the list
	// Also used for the regexps.
	_override_hosts = make(map[string]int64)

	// Cache DNS responses, the key is the hostname to cache
	_cache     = make(map[string]cache_t)
	_cachelock sync.RWMutex
)

func main() {
	// No command, print help
	if len(os.Args) < 2 {
		usage("global", "")
		return
	}

	args, err := getopt(os.Args[2:], "")
	fatal(err)
	for opt, arg := range args {
		fmt.Printf("%s -> %s", opt, arg)
	}

	switch os.Args[1] {
	case "help":
		if len(os.Args) > 2 {
			usage(os.Args[2], "")
		} else {
			usage("global", "")
		}
	case "version":
		fmt.Println("0.1")
	case "server":
		sources := parseConfig("config")
		listen(sources)
	case "status":
		fmt.Println("TODO")
	case "host":
		fmt.Println("TODO")
	case "regex":
		fmt.Println("TODO")
	default:
		usage("global", "invalid command "+os.Args[1])
	}
}

// Start servers
func listen(sources sources_t) {
	chroot()

	// Setup servers
	listenHttp()
	listenCtl()
	dns_udp, dns_tcp := listenDns()
	defer dns_udp.Shutdown()
	defer dns_tcp.Shutdown()

	// Remove old cache items
	go func() {
		for {
			time.Sleep(5 * time.Minute)

			_cachelock.Lock()
			i := 0
			for name, cache := range _cache {
				// Don't lock stuff too long
				if i > 1000 {
					break
				}

				if time.Now().Unix() > cache.expires {
					delete(_cache, name)
				}
				i += 1
			}
			_cachelock.Unlock()
		}
	}()

	// Wait for all servers to start
	// TODO: This can be better − in fact, I'd prefer to open the sockets much
	// earlier and do other init stuff later so we can drop privileges.
	time.Sleep(1 * time.Second)
	info("servers started")

	// Drop privileges
	drop_privs()

	// Read the hosts information *after* starting the DNS server because we can
	// add hosts from remote sources (and thus needs DNS)
	readSources(sources)

	// Wait for SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}

// Setup chroot() from the information in _config
func chroot() {
	info(fmt.Sprintf("chrooting to %v", _config.chroot))
	fatal(os.MkdirAll(_config.chroot, 0755))
	fatal(os.Chown(_config.chroot, _config.uid, _config.gid))
	fatal(os.Chdir(_config.chroot))
	fatal(syscall.Chroot(_config.chroot))

	// Setup /etc/resolv.conf in the chroot for go's resolver
	err := os.MkdirAll("/etc", 0755)
	fatal(err)
	fp, err := os.Create("/etc/resolv.conf")
	defer fp.Close()
	fp.Write([]byte(fmt.Sprintf("nameserver %s", _config.dns_listen[0])))
}

// The MIT License (MIT)
//
// Copyright © 2016 Martin Tournoij
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// The software is provided "as is", without warranty of any kind, express or
// implied, including but not limited to the warranties of merchantability,
// fitness for a particular purpose and noninfringement. In no event shall the
// authors or copyright holders be liable for any claim, damages or other
// liability, whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or other dealings
// in the software.
