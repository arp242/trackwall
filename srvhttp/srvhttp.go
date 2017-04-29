// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Package srvhttp takes care of the HTTP stuff.
package srvhttp

import (
	"crypto/tls"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"time"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"
	"arp242.net/trackwall/srvdns"
)

const _blocked = `<html><head><title> trackwall %[1]s</title></head><body>
<p>trackwall blocked access to <code>%[1]s</code>. Unblock this domain for:</p>
<ul><li><a href="/$@_allow/10s/%[2]s">ten seconds</a></li>
<li><a href="/$@_allow/1h/%[2]s">an hour</a></li>
<li><a href="/$@_allow/1d/%[2]s">a day</a></li>
<li><a href="/$@_allow/10y/%[2]s">until restart</a></li></ul></body></html>`

const _list = `<html><head><title>trackwall</title></head><body><ul>
<li><a href="/$@_list/config">config</a></li>
<li><a href="/$@_list/hosts">hosts</a></li>
<li><a href="/$@_list/regexps">regexps</a></li>
<li><a href="/$@_list/override">override</a></li>
<li><a href="/$@_list/cache">cache</a></li>
</ul></body></html>`

// Bind the sockets.
func Bind() (listenHTTP, listenHTTPS net.Listener) {
	listenHTTP, err := net.Listen("tcp", cfg.Config.HTTPListen.String())
	msg.Fatal(err)

	listenHTTPS, err = net.Listen("tcp", cfg.Config.HTTPSListen.String())
	msg.Fatal(err)

	return listenHTTP, listenHTTPS
}

// This is tcpKeepAliveListener
type httpListener struct {
	*net.TCPListener
}

func (ln httpListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	err = tc.SetKeepAlive(true)
	if err != nil {
		return
	}
	err = tc.SetKeepAlivePeriod(2 * time.Second)
	if err != nil {
		return
	}
	return tc, nil
}

// Serve HTTP requests.
func Serve(listenHTTP, listenHTTPS net.Listener) {
	go func() {
		srv := &http.Server{Addr: cfg.Config.HTTPListen.String()}
		srv.Handler = &handleHTTP{}
		err := srv.Serve(httpListener{listenHTTP.(*net.TCPListener)})
		msg.Fatal(err)
	}()

	go func() {
		srv := &http.Server{Addr: cfg.Config.HTTPSListen.String()}
		srv.Handler = &handleHTTP{}
		srv.TLSConfig = &tls.Config{GetCertificate: getCert}

		tlsListener := tls.NewListener(httpListener{listenHTTPS.(*net.TCPListener)}, srv.TLSConfig)
		err := srv.Serve(tlsListener)
		msg.Fatal(err)
	}()
}

type handleHTTP struct{}

func (f *handleHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Never cache anything here
	w.Header().Set("Cache-Control", "private, max-age=0, no-cache, must-revalidate")

	host := html.EscapeString(r.Host)
	url := html.EscapeString(strings.TrimLeft(r.URL.Path, "/"))

	// Special $@_ control URL
	if strings.HasPrefix(url, "$@_") {
		f.handleHTTPSpecial(w, r, host, url)
		return
	}

	// TODO: Do something sane with the Content-Type header
	sur, success := findSurrogate(host)
	if success {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, sur)
		return
	}

	// Default blocked text
	// TODO: Not reliable enough...
	if strings.HasSuffix(url, ".js") {
		// Add a comment so it won't give parse errors
		// TODO: Make this a text message, rather than HTML
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, fmt.Sprintf("/*"+_blocked+"*/", host, url))
	} else {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, fmt.Sprintf("/*"+_blocked+"*/", host, url))
	}
}

// Handle "special" $@_ urls
func (f *handleHTTP) handleHTTPSpecial(w http.ResponseWriter, r *http.Request, host, url string) {
	// $@_allow/duration/redirect
	if strings.HasPrefix(url, "$@_allow") {
		params := strings.Split(url, "/")
		//fmt.Println(params)
		secs, err := msg.DurationToSeconds(params[1])
		_ = secs
		if err != nil {
			msg.Warn(err)
			return
		}

		// TODO: Always add the shortest entry from the hosts here
		cfg.Override.Store(host, time.Now().Add(time.Duration(secs)*time.Second).Unix())
		srvdns.Cache.Delete("A "+host, "AAAA "+host)

		// Redirect back to where the user came from
		// TODO: Also add query parameters and such!
		w.Header().Set("Location", "/"+strings.Join(params[2:], "/"))
		w.WriteHeader(http.StatusSeeOther) // TODO: Is this the best 30x header?
		// $@_list/{config,hosts,override}
		/* else if strings.HasPrefix(url, "$@_list") {
		params := strings.Split(url, "/")
		if len(params) < 2 || params[1] == "" {
			fmt.Fprintf(w, _list)
			return
		}

		param := params[1]
		switch param {
		case "config":
			spew.Fdump(w, cfg.Config)
		case "hosts":
			fmt.Fprintf(w, fmt.Sprintf("# Blocking %v hosts\n", cfg.Hosts.Len()))
			cfg.Hosts.Dump(w)
		case "regexps":
			cfg.Regexps.Dump(w)
		case "override":
			cfg.Override.Dump(w)
		case "cache":
			srvdns.Cache.Dump()
		}
		*/
	} else {
		fmt.Fprintf(w, "unknown command: %v", url)
	}
}

// Try to find a surrogate.
func findSurrogate(host string) (script string, success bool) {
	// Exact match! Hurray! This is fastest.
	sur, exists := cfg.Hosts.Get(host)
	if exists && sur != "" {
		return sur, true
	}

	// Slower check if a regex matches the domain
	return cfg.Surrogates.Match(host)
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
