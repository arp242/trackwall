// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// The DNS stuff
package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Setup DNS server
func listenDns() (dns.Server, dns.Server) {
	dns.HandleFunc(".", handleDns)
	dns_udp := dns.Server{Addr: _config.dns_listen.String(), Net: "udp"}
	go func() {
		err := dns_udp.ListenAndServe()
		fatal(err)
	}()
	dns_tcp := dns.Server{Addr: _config.dns_listen.String(), Net: "tcp"}
	go func() {
		err := dns_tcp.ListenAndServe()
		fatal(err)
	}()

	return dns_udp, dns_tcp
}

// Handle a DNS request
func handleDns(w dns.ResponseWriter, req *dns.Msg) {
	for {
		if !_config.locked {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Make sure we're using the correct uid in the callback; this is sometimes
	// still 0 on the first few requests due to the Setresuid() being called
	// after the goroutines that start the servers.
	drop_privs()

	if len(req.Question) == 0 {
		dns.HandleFailed(w, req)
		return
	}

	// We only need to spoof A and AAAA records
	t := dns.TypeToString[req.Question[0].Qtype]
	if t != "A" && t != "AAAA" {
		forward(_config.dns_forward.String(), w, req)
		return
	}

	name := strings.TrimRight(req.Question[0].Name, ".")
	var response uint8

	// Check cache
	_cachelock.Lock()
	cache, have_cache := _cache[name]
	if have_cache && cache.expires > time.Now().Unix() {
		response = cache.response
	} else {
		response = determineResponse(name, t)
		have_cache = false

		// TODO: Also cache ipv6
		if t != "AAAA" {
			// TODO: Don't hard-code the cache time to one hour.
			_cache[name] = cache_t{expires: time.Now().Unix() + 3600, response: response}
		}
	}
	_cachelock.Unlock()

	//fmt.Printf("cache size: %v (about %v bytes)\n", len(_cache), len(_cache)*6)

	switch response {
	case RESPONSE_FORWARD:
		if !have_cache {
			info(fmt.Sprintf("forward  %v", name))
		}
		forward(_config.dns_forward.String(), w, req)
	case RESPONSE_SPOOF:
		if !have_cache {
			info(fmt.Sprintf("spoof    %v", name))
		}
		spoof(name, w, req)
	case RESPONSE_NXDOMAIN:
		if !have_cache {
			//info(fmt.Sprintf("nxdomain %v", name))
		}
		spoofNxdomain(name, w, req)
	}
}

// Determine what to do with the hostname name. Returns a RESPONSE_* constant.
func determineResponse(name, t string) uint8 {
	var do_spoof bool

	// TODO: This doesn't work very well since browsers cache the shit out of
	// DNS; so we may want to use a HTTP proxy for this?
	expires, have_override := _override_hosts[name]
	if have_override {
		if time.Now().Unix() > expires {
			delete(_override_hosts, name)
			have_override = false
		} else {
			do_spoof = false
		}
	}

	if !have_override {
		_, do_spoof = _hosts[name]
		if !do_spoof {
			for _, r := range _regexps {
				if r.MatchString(name) {
					do_spoof = true
					break
				}
			}
		}
	}

	// For now, we just pretend that AAAA records that we want to spoof don't
	// exist (NXDOMAIN).
	// TODO: This could be better, but I'm not sure how to properly do this. We
	// now listen on 127.0.0.53 to prevent interfering with existing DNS
	// daemons, HTTP servers, etc. (/etc/resolv.conf doesn't support adding a
	// port number). IPv6 only has one loopback address (::1) and nota /8 like
	// IPv4...
	if do_spoof && t == "AAAA" {
		return RESPONSE_NXDOMAIN
	} else if do_spoof {
		return RESPONSE_SPOOF
	} else {
		return RESPONSE_FORWARD
	}
}

// Spoof DNS response by replying with the address of our HTTP server. This only
// does A records.
func spoof(name string, w dns.ResponseWriter, req *dns.Msg) {
	spec := fmt.Sprintf("%s. 1 IN A %s", name, _config.http_listen.host)
	rr, err := dns.NewRR(spec)
	fatal(err)

	var msg dns.Msg
	msg.MsgHdr.Id = req.MsgHdr.Id
	msg.MsgHdr.Response = true
	msg.MsgHdr.RecursionDesired = true
	msg.MsgHdr.RecursionAvailable = true
	msg.Question = req.Question
	msg.Answer = []dns.RR{rr}
	msg.Ns = []dns.RR{}
	msg.Extra = []dns.RR{}

	w.WriteMsg(&msg)
}

// Spoof DNS response by replying with the NXDOMAIN rcode (and no answer).
func spoofNxdomain(name string, w dns.ResponseWriter, req *dns.Msg) {
	var msg dns.Msg
	msg.MsgHdr.Id = req.MsgHdr.Id
	msg.MsgHdr.Response = true
	msg.MsgHdr.RecursionDesired = true
	msg.MsgHdr.RecursionAvailable = true
	//msg.MsgHdr.Rcode = dns.RcodeNameError
	msg.Question = req.Question
	msg.Answer = []dns.RR{}
	msg.Ns = []dns.RR{}
	msg.Extra = []dns.RR{}

	w.WriteMsg(&msg)
}

// Forward DNS request to forward-dns
func forward(addr string, w dns.ResponseWriter, req *dns.Msg) {
	transport := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		transport = "tcp"
	}
	c := &dns.Client{Net: transport}
	resp, _, err := c.Exchange(req, addr)
	if err != nil {
		dns.HandleFailed(w, req)
		warn(fmt.Errorf("unable to forward DNS request to %v: %v", addr, err))
		return
	}

	w.WriteMsg(resp)
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
