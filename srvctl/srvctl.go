// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Package srvctl contains the control socket
package srvctl

import (
	"bufio"
	"fmt"
	"io"
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

	input, isHTTP, err := readCommand(conn)
	if err != nil {
		msg.Warn(err)
		return
	}

	var w string
	switch input[0] {
	case "":
		if isHTTP {
			conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n"))
			w = tplIndex
		} else {
			w = fmt.Sprintf("error: unknown command: %#v", input[0])
		}
	case "status":
		if len(input) < 2 {
			w = "error: need a subcommand"
		} else {
			w = handleStatus(input[1], conn)
		}
	case "cache":
		if len(input) < 2 {
			w = "error: need a subcommand"
		} else {
			w = handleCache(input[1], conn)
		}
	case "override":
		if len(input) < 2 {
			w = "error: need a subcommand"
		} else {
			w = handleOverride(input[1], conn)
		}
	case "host":
	case "regex":
	default:
		w = fmt.Sprintf("error: unknown command: %#v", input[0])
	}

	fmt.Fprintf(conn, w+"\n")
	msg.Warn(err)
}

func readCommand(conn io.Reader) (
	input []string,
	isHTTP bool,
	err error,
) {

	data, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return nil, false, err
	}

	sdata := string(data)

	// This accepts simple "telnet" style commands:
	//   status summary
	//   host add example.com example2.com
	//
	// But we also accept HTTP-style:
	//   GET /status/summary HTTP/1.1\r\n
	//   GET /host/add/example.com/example2.com HTTP/1.1\r\n"
	if strings.HasPrefix(sdata, "GET /") {
		// Remove GET and HTTP/1.1\r\n
		sdata = sdata[5 : len(sdata)-11]
		input = strings.Split(strings.TrimSpace(sdata), "/")
		isHTTP = true
	} else {
		input = strings.Split(strings.TrimSpace(sdata), " ")
	}

	return input, isHTTP, nil
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
