// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// The control socket
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"runtime"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

func bindCtl() net.Listener {
	l, err := net.Listen("tcp", _config.ControlListen.String())
	fatal(err)
	return l
}

func setupCtlHandle(l net.Listener) {
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				warn(err)
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
		warn(err)
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
	case "log":
	default:
		w = fmt.Sprintf("error: unknown command: %#v", data)
	}

	fmt.Fprintf(conn, w+"\n")
	warn(err)
}

func handleCache(cmd string, w net.Conn) (out string) {
	switch cmd {
	case "flush":
		_cachelock.Lock()
		_cache = make(map[string]cacheT)
		_cachelock.Unlock()
	default:
		out = "error: unknown subcommand"
	}

	return out
}

func handleOverride(cmd string, w net.Conn) (out string) {
	switch cmd {
	case "flush":
		// TODO: lock!
		_overrideHosts = make(map[string]int64)
	default:
		out = "error: unknown subcommand"
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

		_hostsLock.Lock()
		fmt.Fprintf(w, "hosts:             %v\n", len(_hosts))
		_hostsLock.Unlock()
		_regexpsLock.Lock()
		fmt.Fprintf(w, "regexps:           %v\n", len(_regexps))
		_regexpsLock.Unlock()
		fmt.Fprintf(w, "cache items:       %v\n", len(_cache))
		fmt.Fprintf(w, "memory allocated:  %vKb\n", stats.Sys/1024)
	case "config":
		scs.Fdump(w, _config)
	case "cache":
		_cachelock.Lock()
		scs.Fdump(w, _cache)
		_cachelock.Unlock()
	case "hosts":
		fmt.Fprintf(w, fmt.Sprintf("# Blocking %v hosts\n", len(_hosts)))
		_hostsLock.Lock()
		for k, v := range _hosts {
			if v != "" {
				fmt.Fprintf(w, fmt.Sprintf("%v  # %v\n", k, v))
			} else {
				fmt.Fprintf(w, fmt.Sprintf("%v\n", k))
			}
		}
		_hostsLock.Unlock()
	case "regexps":
		_regexpsLock.Lock()
		for _, v := range _regexps {
			fmt.Fprintf(w, fmt.Sprintf("%v\n", v))
		}
		_regexpsLock.Unlock()
	case "override":
		scs.Fdump(w, _overrideHosts)
	default:
		out = "error: unknown subcommand"
	}

	return out
}

func writeCtl(what string) {
	conn, err := net.Dial("tcp", _config.ControlListen.String())
	fatal(err)
	defer func() { _ = conn.Close() }()

	fmt.Fprintf(conn, what+"\n")
	data, err := ioutil.ReadAll(conn)
	fatal(err)
	fmt.Println(strings.TrimSpace(string(data)))
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
