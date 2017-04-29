// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Package srvctl contains the control socket
package srvctl

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"runtime"
	"strings"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"
	"arp242.net/trackwall/srvdns"

	"github.com/davecgh/go-spew/spew"
)

// Bind the socket.
func Bind() net.Listener {
	l, err := net.Listen("tcp", cfg.Config.ControlListen.String())
	msg.Fatal(err)
	return l
}

// Serve requests.
func Serve(l net.Listener) {
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				msg.Warn(err)
				continue
			}
			go handleCtl(conn)
		}
	}()
}

func handleCtl(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	data, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		msg.Warn(err)
		return
	}

	sdata := string(data)

	// This accepts simple "telnet" style commands:
	// status summary
	// host add example.com example2.com
	//
	// But we also accept HTTP-style:
	// GET /status/summary HTTP/1.1\r\n
	// GET /host/add/example.com/example2.com HTTP/1.1\r\n"
	var ldata []string
	if strings.HasPrefix(sdata, "GET /") {
		// Remove GET and HTTP/1.1\r\n
		sdata = sdata[5 : len(sdata)-11]
		ldata = strings.Split(strings.TrimSpace(sdata), "/")
	} else {
		ldata = strings.Split(strings.TrimSpace(sdata), " ")
	}

	var w string
	switch ldata[0] {
	case "status":
		if len(ldata) < 2 {
			w = "error: need a subcommand"
		} else {
			w = handleStatus(ldata[1], conn)
		}
	case "cache":
		if len(ldata) < 2 {
			w = "error: need a subcommand"
		} else {
			w = handleCache(ldata[1], conn)
		}
	case "override":
		if len(ldata) < 2 {
			w = "error: need a subcommand"
		} else {
			w = handleOverride(ldata[1], conn)
		}
	case "host":
	case "regex":
	default:
		w = fmt.Sprintf("error: unknown command: %#v", data)
	}

	fmt.Fprintf(conn, w+"\n")
	msg.Warn(err)
}

func handleCache(cmd string, w net.Conn) (out string) {
	switch cmd {
	case "flush":
		srvdns.Cache.Purge()
		out = "okay"
	default:
		out = fmt.Sprintf("error: unknown subcommand: %#v", cmd)
	}

	return out
}

func handleOverride(cmd string, w net.Conn) (out string) {
	switch cmd {
	case "flush":
		cfg.Override.Purge()
		out = "okay"
	default:
		out = fmt.Sprintf("error: unknown subcommand: %#v", cmd)
	}

	return out
}

func handleStatus(cmd string, w net.Conn) (out string) {
	scs := spew.ConfigState{Indent: "\t"}

	switch cmd {
	case "summary":
		var stats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&stats)

		fmt.Fprintf(w, "hosts:             %v\n", cfg.Hosts.Len())
		fmt.Fprintf(w, "regexps:           %v\n", cfg.Regexps.Len())
		fmt.Fprintf(w, "cache items:       %v\n", srvdns.Cache.Len())
		fmt.Fprintf(w, "memory allocated:  %vKb\n", stats.Sys/1024)
	case "config":
		scs.Fdump(w, cfg.Config)
	case "cache":
		srvdns.Cache.Dump(w)
	case "hosts":
		fmt.Fprintf(w, fmt.Sprintf("# Blocking %v hosts\n", cfg.Hosts.Len()))
		cfg.Hosts.Dump(w)
	case "regexps":
		cfg.Regexps.Dump(w)
	case "override":
		cfg.Override.Dump(w)
	default:
		out = fmt.Sprintf("error: unknown subcommand: %#v", cmd)
	}

	return out
}

// Write to the server.
func Write(what string) {
	conn, err := net.Dial("tcp", cfg.Config.ControlListen.String())
	msg.Fatal(err)
	defer func() { _ = conn.Close() }()

	fmt.Fprintf(conn, what+"\n")
	data, err := ioutil.ReadAll(conn)
	msg.Fatal(err)
	fmt.Println(strings.TrimSpace(string(data)))
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
