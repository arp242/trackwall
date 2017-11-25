// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"
	"arp242.net/trackwall/srvctl"
	"arp242.net/trackwall/srvdns"
	"arp242.net/trackwall/srvhttp"
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run as DNS/HTTP server",
	Long: `
Start the DNS and HTTP(S) server. This is the main operation of the program.
Almost all options are controlled through the configuration file.

Note that trackwall cannot run as a "daemon", and we assume that the system
provides some mechanism to cope with this (many do, such as daemontools, runit,
systemd, upstart, etc).
For systems that don't provide this, you'll need to use a wrapper.
`,
	Run: func(cmd *cobra.Command, args []string) {
		listen()
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
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
	defer dnsUDP.Shutdown() // nolint: errcheck
	defer dnsTCP.Shutdown() // nolint: errcheck

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
