// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// DNS proxy to spoof responses in order to block ads and malicious websites.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/cmdline"
	"arp242.net/trackwall/msg"
	"arp242.net/trackwall/srvctl"
	"arp242.net/trackwall/srvdns"
	"arp242.net/trackwall/srvhttp"
)

func main() {
	if len(os.Args) < 2 {
		cmdline.Usage("global", "")
		os.Exit(0)
	}
	commands, config, verbose, err := cmdline.Process(os.Args[1:])
	msg.Fatal(err)
	cfg.Config.Verbose = verbose
	if len(commands) == 0 {
		cmdline.Usage("global", "")
		os.Exit(0)
	}

	err = cfg.Load(config)
	if err != nil {
		msg.Fatal(fmt.Errorf("cannot load config: %v", err))
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
		srvctl.Write(strings.Join(commands, " "))
	case "cache":
		if len(os.Args) < 2 {
			cmdline.Usage("cache", "cache needs a command")
		}
		srvctl.Write(strings.Join(commands, " "))
	case "override":
		if len(os.Args) < 2 {
			cmdline.Usage("override", "override needs a command")
		}
		srvctl.Write(strings.Join(commands, " "))
	case "host":
		if len(os.Args) < 2 {
			cmdline.Usage("override", "override needs a command")
		}
		srvctl.Write(strings.Join(commands, " "))
	case "regex":
		if len(os.Args) < 2 {
			cmdline.Usage("override", "override needs a command")
		}
		srvctl.Write(strings.Join(commands, " "))
	default:
		srvctl.Write(fmt.Sprintf("error: unknown command: %#v", commands[0]))
	}
}

func compile() {
	chroot()
	DropPrivs()

	os.Remove("/cache/compiled")
	cfg.Config.ReadHosts()
	cfg.Config.Compile()
}

// Start servers
func listen() {
	chroot()

	// Setup servers; the bind* function only sets up the socket.
	ctl := srvctl.Bind()
	http, https := srvhttp.Bind()
	dnsUDP, dnsTCP := srvdns.Serve(cfg.Config.DNSListen.String(),
		cfg.Config.DNSForward.String(), cfg.Config.CacheDNS, cfg.Config.HTTPListen.Host,
		cfg.Config.Verbose)
	defer dnsUDP.Shutdown()
	defer dnsTCP.Shutdown()

	// Wait for the servers to start.
	// TODO: This should be better.
	time.Sleep(2 * time.Second)

	// Drop privileges
	DropPrivs()

	srvctl.Serve(ctl)
	srvhttp.Serve(http, https)

	// Read the hosts information *after* starting the DNS server because we can
	// add hosts from remote sources (and thus needs DNS)
	cfg.Config.ReadHosts()

	msg.Info("initialisation finished; ready to serve", cfg.Config.Verbose)

	// Wait for SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}

// Setup chroot() from the information in cfg.Config
func chroot() {
	msg.Info(fmt.Sprintf("chrooting to %v", cfg.Config.Chroot), cfg.Config.Verbose)

	// Make sure the chroot dir exists with the correct permissions and such
	_, err := os.Stat(cfg.Config.Chroot)
	if os.IsNotExist(err) {
		msg.Warn(fmt.Errorf("chroot dir %s doesn't exist, attempting to create", cfg.Config.Chroot))
		msg.Fatal(os.MkdirAll(cfg.Config.Chroot, 0755))
		msg.Fatal(os.Chown(cfg.Config.Chroot, cfg.Config.User.UID, cfg.Config.User.GID))
	}

	// TODO: We do this *before* the chroot since on OpenBSD it needs access to
	// /dev/urandom, which we don't have in the chroot (and I'd rather not add
	// this as a dependency).
	// This should be fixed in Go 1.7 by using getentropy() (see #13785, #14572)
	if _, err := os.Stat(cfg.Config.ChrootDir(cfg.Config.RootKey)); os.IsNotExist(err) {
		srvhttp.MakeRootKey()
	}
	if _, err := os.Stat(cfg.Config.ChrootDir(cfg.Config.RootCert)); os.IsNotExist(err) {
		srvhttp.MakeRootCert()
	}

	msg.Fatal(os.Chdir(cfg.Config.Chroot))
	err = syscall.Chroot(cfg.Config.Chroot)
	if err != nil {
		msg.Fatal(fmt.Errorf("unable to chroot to %v: %v", cfg.Config.Chroot, err.Error()))
	}

	// Setup /etc/resolv.conf in the chroot for Go's resolver
	err = os.MkdirAll("/etc", 0755)
	msg.Fatal(err)
	fp, err := os.Create("/etc/resolv.conf")
	defer fp.Close()
	fp.Write([]byte(fmt.Sprintf("nameserver %s", cfg.Config.DNSListen.Host)))

	// Make sure the rootCA files exist and are not world-readable.
	keyfile := func(path string) string {
		st, err := os.Stat(path)
		msg.Fatal(err)

		if st.Mode().Perm().String() != "-rw-------" {
			msg.Warn(fmt.Errorf("insecure permissions for %s, attempting to fix", path))
			msg.Fatal(os.Chmod(path, os.FileMode(0600)))
		}

		err = os.Chown(path, cfg.Config.User.UID, cfg.Config.User.GID)
		msg.Fatal(err)
		return path
	}

	cfg.Config.RootKey = keyfile(cfg.Config.RootKey)
	cfg.Config.RootCert = keyfile(cfg.Config.RootCert)
}

// DropPrivs drops to an unpriviliged user.
func DropPrivs() {
	// TODO Don't do this on Linux systems for now.
	//
	// Calls to this are peppered throughout since on Linux different threads can
	// have a different uid/gid, and the syscall only sets it for the *current*
	// thread.
	// See: https://github.com/golang/go/issues/1435
	//
	// This is only an issue on Linux, not on other systems.
	//
	// This is really a quick stop-hap solution and we should do this better. One
	// way is to start a new process after the privileged initialisation and pass
	// the filenos to that, but that would require reworking quite a bit of the DNS
	// server bits in the dns package...
	//
	// setuidgid(8) should work
	if runtime.GOOS == "linux" {
		return
	}

	msg.Info("dropping privileges", cfg.Config.Verbose)

	err := syscall.Setresgid(cfg.Config.User.GID, cfg.Config.User.GID, cfg.Config.User.GID)
	msg.Fatal(err)
	err = syscall.Setresuid(cfg.Config.User.UID, cfg.Config.User.UID, cfg.Config.User.UID)
	msg.Fatal(err)

	// Double-check just to be sure.
	if syscall.Getuid() != cfg.Config.User.UID || syscall.Getgid() != cfg.Config.User.GID {
		msg.Fatal(fmt.Errorf("unable to drop privileges"))
	}
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
