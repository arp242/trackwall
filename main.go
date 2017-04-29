// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// DNS proxy to spoof responses in order to block ads and malicious websites.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"arp242.net/trackwall/cmdline"
)

const (
	reponseForward = 1
	reponseSpoof   = 2
	reponseEmty    = 3
)

// Cache entry
type cacheT struct {
	// We don't cache the actual DNS responses − that's the resolver's job. We
	// just cache the action taken. That's enough and saves some time in
	// processing regexps and such
	// See the RESPONSE_* constants for the possible values.
	response uint8
	expires  int64
}

var (
	_config ConfigT

	// Static hosts added with hostlist/host. The key is the hostname, the
	// (optional) value is a surrogate script to serve.
	_hosts     = make(map[string]string)
	_hostsLock sync.RWMutex

	_surrogates     []surrogateT
	_surrogatesLock sync.RWMutex

	// Compiled regexes added with regexlist/regex. Pre-compiling the surrogate
	// scripts isn't possible here.
	_regexps     []*regexp.Regexp
	_regexpsLock sync.RWMutex

	// Hosts to override; value is timestamp, once that's expired the entry will be
	// removed from the list
	// Also used for the regexps.
	_overrideHosts     = make(map[string]int64)
	_overrideHostsLock sync.RWMutex

	// Cache DNS responses, the key is the hostname to cache
	_cache     = make(map[string]cacheT)
	_cachelock sync.RWMutex

	// Print more info to the screen
	_verbose = false
)

func main() {
	info("starting trackwall")

	if len(os.Args) < 2 {
		cmdline.Usage("global", "")
		os.Exit(0)
	}
	commands, config, verbose, err := cmdline.Process(os.Args[1:])
	fatal(err)
	_verbose = verbose
	if len(commands) == 0 {
		cmdline.Usage("global", "")
		os.Exit(0)
	}

	err = loadConfig(config)
	if err != nil {
		fatal(fmt.Errorf("cannot load %v: %v", config, err))
	}

	switch commands[0] {
	case "help":
		if len(commands) > 1 {
			cmdline.Usage(commands[1], "")
		} else {
			cmdline.Usage("global", "")
		}
	case "version":
		if len(commands) > 1 {
			cmdline.Usage("version", "version does not accept commands")
		}
		fmt.Println("0.1")
	case "server":
		if len(commands) > 1 {
			cmdline.Usage("version", "server does not accept commands")
		}
		listen()
	case "compile":
		if len(commands) > 1 {
			cmdline.Usage("version", "compile does not accept commands")
		}
		compile()
	case "status":
		if len(commands) < 2 {
			cmdline.Usage("status", "status needs a command")
		}
		writeCtl(strings.Join(commands, " "))
	case "cache":
		if len(os.Args) < 2 {
			cmdline.Usage("cache", "cache needs a command")
		}
		writeCtl(strings.Join(commands, " "))
	case "override":
		if len(os.Args) < 2 {
			cmdline.Usage("override", "override needs a command")
		}
		writeCtl(strings.Join(commands, " "))
	case "host":
		if len(os.Args) < 2 {
			cmdline.Usage("override", "override needs a command")
		}
		writeCtl(strings.Join(commands, " "))
	case "regex":
		if len(os.Args) < 2 {
			cmdline.Usage("override", "override needs a command")
		}
		writeCtl(strings.Join(commands, " "))
	default:
		writeCtl(fmt.Sprintf("error: unknown command: %#v", commands[0]))
	}
}

func compile() {
	chroot()
	dropPrivs()

	os.Remove("/cache/compiled")
	_config.readHosts()
	_config.compile()
}

// Start servers
func listen() {
	chroot()

	// Setup servers; the bind* function only sets up the socket.
	ctl := bindCtl()
	http, https := bindHTTP()
	dnsUDP, dnsTCP := listenDNS()
	defer dnsUDP.Shutdown()
	defer dnsTCP.Shutdown()

	// Wait for the servers to start.
	// TODO: This should be better.
	time.Sleep(2 * time.Second)

	// Drop privileges
	dropPrivs()

	setupCtlHandle(ctl)
	setupHTTPHandle(http, https)

	// Read the hosts information *after* starting the DNS server because we can
	// add hosts from remote sources (and thus needs DNS)
	_config.readHosts()

	// Remove old cache items every 5 minutes.
	startCachePurger()

	info("initialisation finished; ready to serve")

	// Wait for SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}

// Setup chroot() from the information in _config
func chroot() {
	info(fmt.Sprintf("chrooting to %v", _config.Chroot))

	// Make sure the chroot dir exists with the correct permissions and such
	_, err := os.Stat(_config.Chroot)
	if os.IsNotExist(err) {
		warn(fmt.Errorf("chroot dir %s doesn't exist, attempting to create", _config.Chroot))
		fatal(os.MkdirAll(_config.Chroot, 0755))
		fatal(os.Chown(_config.Chroot, _config.User.UID, _config.User.GID))
	}

	// TODO: We do this *before* the chroot since on OpenBSD it needs access to
	// /dev/urandom, which we don't have in the chroot (and I'd rather not add
	// this as a dependency).
	// This should be fixed in Go 1.7 by using getentropy() (see #13785, #14572)
	if _, err := os.Stat(chrootdir(_config.RootKey)); os.IsNotExist(err) {
		makeRootKey()
	}
	if _, err := os.Stat(chrootdir(_config.RootCert)); os.IsNotExist(err) {
		makeRootCert()
	}

	fatal(os.Chdir(_config.Chroot))
	err = syscall.Chroot(_config.Chroot)
	if err != nil {
		fatal(fmt.Errorf("unable to chroot to %v: %v", _config.Chroot, err.Error()))
	}

	// Setup /etc/resolv.conf in the chroot for Go's resolver
	err = os.MkdirAll("/etc", 0755)
	fatal(err)
	fp, err := os.Create("/etc/resolv.conf")
	defer fp.Close()
	fp.Write([]byte(fmt.Sprintf("nameserver %s", _config.DNSListen.Host)))

	// Make sure the rootCA files exist and are not world-readable.
	keyfile := func(path string) string {
		st, err := os.Stat(path)
		fatal(err)

		if st.Mode().Perm().String() != "-rw-------" {
			warn(fmt.Errorf("insecure permissions for %s, attempting to fix", path))
			fatal(os.Chmod(path, os.FileMode(0600)))
		}

		err = os.Chown(path, _config.User.UID, _config.User.GID)
		fatal(err)
		return path
	}

	_config.RootKey = keyfile(_config.RootKey)
	_config.RootCert = keyfile(_config.RootCert)
}

// The MIT License (MIT)
//
// Copyright © 2016-2017 Martin Tournoij
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
