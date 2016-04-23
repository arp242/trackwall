// The HTTP stuff
//
// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.
package main

import (
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

const _blocked = `<html><head><title>dnsblock %[1]s</title></head><body>
<p>dnsblock blocked access to <code>%[1]s</code>. Unblock this domain for:</p>
<ul><li><a href="/$@_allow/10s/%[2]s">ten seconds</a></li>
<li><a href="/$@_allow/1h/%[2]s">an hour</a></li>
<li><a href="/$@_allow/1d/%[2]s">a day</a></li>
<li><a href="/$@_allow/10y/%[2]s">until restart</a></li></ul></body></html>`

const _list = `<html><head><title>dnsblock</title></head><body><ul>
<li><a href="/$@_list/config">config</a></li>
<li><a href="/$@_list/hosts">hosts</a></li>
<li><a href="/$@_list/override">override</a></li>
<li><a href="/$@_list/cache">cache</a></li>
</ul></body></html>`

func listenHttp() {
	go func() {
		err := http.ListenAndServe(joinAddr(_config.http_listen), &handleHttp{})
		fatal(err)
	}()
	go func() {
		// TODO: we should generate a cert on connection with the correct
		// hostname signed by a dnsblock provided root certicicate.
		// go run /usr/share/go/src/crypto/tls/generate_cert.go --host 127.0.0.53 --duration 20y
		err := http.ListenAndServeTLS(joinAddr(_config.https_listen),
			_config.https_cert, _config.https_key, &handleHttp{})
		fatal(err)
	}()
}

type handleHttp struct{}

func (f *handleHttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// See comment in handleDns
	drop_privs()

	host := html.EscapeString(r.Host)
	url := html.EscapeString(strings.TrimLeft(r.URL.Path, "/"))

	// Special $@_ control URL
	if strings.HasPrefix(url, "$@_") {
		// $@_allow/duration/redirect
		if strings.HasPrefix(url, "$@_allow") {
			params := strings.Split(url, "/")
			fmt.Println(params)
			secs, err := durationToSeconds(params[1])
			if err != nil {
				warn(err)
				return
			}

			_override_hosts[host] = time.Now().Add(time.Duration(secs) * time.Second).Unix()
			w.Header().Set("Location", "/"+strings.Join(params[2:], "/"))
			w.WriteHeader(http.StatusSeeOther) // TODO: Is this the best 30x header?
			// $@_list/{config,hosts,override}
		} else if strings.HasPrefix(url, "$@_list") {
			params := strings.Split(url, "/")
			if len(params) < 2 || params[1] == "" {
				fmt.Fprintf(w, _list)
				return
			}

			param := params[1]
			if param == "config" {
				fmt.Fprintf(w, fmt.Sprintf("%#v", _config))
			} else if param == "hosts" {
				fmt.Fprintf(w, fmt.Sprintf("# Blocking %v hosts\n", len(_hosts)))
				for k, v := range _hosts {
					if v != "" {
						fmt.Fprintf(w, fmt.Sprintf("%v  # %v\n", k, v))
					} else {
						fmt.Fprintf(w, fmt.Sprintf("%v\n", k))
					}
				}
			} else if param == "override" {
				fmt.Fprintf(w, fmt.Sprintf("%#v", _override_hosts))
			} else if param == "cache" {
				fmt.Fprintf(w, fmt.Sprintf("%#v", _cache))
			}
		} else {
			fmt.Fprintf(w, "unknown command: %v", url)
		}
	} else {
		// Check for surrogate script
		sur, exists := _hosts[host]

		// This should never happen
		// TODO: this does happen if we block by regexp; we need to do special
		// foo here
		if !exists {
			warn(fmt.Errorf("host %v not in _hosts?!", host))
		}

		// TODO: Do something sane with the Content-Type header
		if sur != "" {
			fmt.Fprintf(w, sur)
		} else {
			fmt.Fprintf(w, fmt.Sprintf(_blocked, host, url))
		}
	}
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
