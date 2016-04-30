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
// TODO: Splitting out the binding of the socket and starting a server is not
// easy, so we don't...
func listenDns() (*dns.Server, *dns.Server) {
	dns.HandleFunc(".", handleDns)

	dns_udp := &dns.Server{Addr: _config.dns_listen.String(), Net: "udp"}
	go func() {
		err := dns_udp.ListenAndServe()
		fatal(err)
	}()

	dns_tcp := &dns.Server{Addr: _config.dns_listen.String(), Net: "tcp"}
	go func() {
		err := dns_tcp.ListenAndServe()
		fatal(err)
	}()

	return dns_udp, dns_tcp
}

// Handle a DNS request
func handleDns(w dns.ResponseWriter, req *dns.Msg) {
	name := strings.TrimRight(req.Question[0].Name, ".")

	// Wait until _hosts are loaded, except when downloading the host lists
	if !_config.sources.hasDomain(name) {
		for {
			if !_config.locked {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

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

	response, from_cache := getResponse(name, t)
	switch response {
	case RESPONSE_FORWARD:
		if !from_cache {
			//info(fmt.Sprintf("%sorward  %v", greenbg("f"), name))
			infoc(fmt.Sprintf("forward  %v", name), "green")
		}
		forward(_config.dns_forward.String(), w, req)
	case RESPONSE_SPOOF:
		if !from_cache {
			//info(fmt.Sprintf("%spoof    %v", orangebg("s"), name))
			infoc(fmt.Sprintf("spoof    %v", name), "orange")
		}
		spoof(name, w, req)
	case RESPONSE_EMPTY:
		if !from_cache {
			//info(fmt.Sprintf("empty  %v", name))
		}
		spoofEmpty(w, req)
	}
}

// Get response from cache (if it exists and is not expired), or determine a new
// response.
func getResponse(name, t string) (response uint8, from_cache bool) {
	// First check override
	// TODO: It might be better/faster to clear cache entries when adding
	// override?
	if haveOverride(name) {
		return RESPONSE_FORWARD, false
	}

	cachekey := t + " " + name
	_cachelock.Lock()
	cache, have_cache := _cache[cachekey]
	_cachelock.Unlock()

	if have_cache && cache.expires > time.Now().Unix() {
		return cache.response, true
	}

	response = determineResponse(name, t)

	_cachelock.Lock()
	_cache[cachekey] = cache_t{
		expires:  time.Now().Unix() + int64(_config.cache_dns),
		response: response,
	}
	_cachelock.Unlock()

	return response, false
}

func haveOverride(name string) bool {
	// Check override
	expires, have_override := _override_hosts[name]

	if !have_override {
		labels := strings.Split(name, ".")
		c := ""
		l := len(labels)
		for i := 0; i < l; i += 1 {
			if c == "" {
				c = labels[l-i-1]
			} else {
				c = labels[l-i-1] + "." + c
			}

			expires, have_override = _override_hosts[c]
			if have_override {
				break
			}
		}
	}

	// Make sure it's not expires
	if have_override {
		if time.Now().Unix() > expires {
			delete(_override_hosts, name)
			have_override = false
		}
	}

	return have_override
}

// Determine what to do with the hostname name. Returns a RESPONSE_* constant.
func determineResponse(name, t string) uint8 {
	var do_spoof bool

	have_override := haveOverride(name)
	if have_override {
		do_spoof = false
	}

	if !have_override {
		// Hosts
		labels := strings.Split(name, ".")
		c := ""
		l := len(labels)
		for i := 0; i < l; i += 1 {
			if c == "" {
				c = labels[l-i-1]
			} else {
				c = labels[l-i-1] + "." + c
			}

			_, do_spoof = _hosts[c]
			if do_spoof {
				break
			}
		}

		// Regexps
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
	// exist (EMPTY).
	// TODO: This could be better, but I'm not sure how to properly do this. We
	// now listen on 127.0.0.53 to prevent interfering with existing DNS
	// daemons, HTTP servers, etc. (/etc/resolv.conf doesn't support adding a
	// port number). IPv6 only has one loopback address (::1) and nota /8 like
	// IPv4...
	if do_spoof && t == "AAAA" {
		return RESPONSE_EMPTY
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

	sendSpoof([]dns.RR{rr}, w, req)
}

// Spoof DNS response by replying with an empty answer section.
func spoofEmpty(w dns.ResponseWriter, req *dns.Msg) {
	sendSpoof([]dns.RR{}, w, req)
}

// Make a message with the answer and write it to the client
func sendSpoof(answer []dns.RR, w dns.ResponseWriter, req *dns.Msg) {
	var msg dns.Msg
	msg.MsgHdr.Id = req.MsgHdr.Id
	msg.MsgHdr.Response = true
	msg.MsgHdr.RecursionDesired = true
	msg.MsgHdr.RecursionAvailable = true
	msg.Question = req.Question
	msg.Answer = answer
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
		warn(fmt.Errorf("unable to forward DNS request for %v to %v: %v",
			req.Question[0], addr, err))
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
