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
func listenDNS() (*dns.Server, *dns.Server) {
	dns.HandleFunc(".", handleDNS)

	dnsUDP := &dns.Server{Addr: _config.DNSListen.String(), Net: "udp"}
	go func() {
		err := dnsUDP.ListenAndServe()
		fatal(err)
	}()

	dnsTCP := &dns.Server{Addr: _config.DNSListen.String(), Net: "tcp"}
	go func() {
		err := dnsTCP.ListenAndServe()
		fatal(err)
	}()

	return dnsUDP, dnsTCP
}

// Handle a DNS request
func handleDNS(w dns.ResponseWriter, req *dns.Msg) {
	name := strings.TrimRight(req.Question[0].Name, ".")

	// Wait until _hosts are loaded, except when downloading the host lists
	if !_config.hasDomain(name) {
		for {
			if !_config.Locked {
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
		forward(_config.DNSForward.String(), w, req)
		return
	}

	response, fromCache := getResponse(name, t)
	switch response {
	case reponseForward:
		if !fromCache {
			//info(fmt.Sprintf("%sorward  %v", greenbg("f"), name))
			infoc(fmt.Sprintf("forward  %v", name), "green")
		}
		forward(_config.DNSForward.String(), w, req)
	case reponseSpoof:
		if !fromCache {
			//info(fmt.Sprintf("%spoof    %v", orangebg("s"), name))
			infoc(fmt.Sprintf("spoof    %v", name), "orange")
		}
		spoof(name, w, req)
	case reponseEmty:
		if !fromCache {
			//info(fmt.Sprintf("empty  %v", name))
		}
		spoofEmpty(w, req)
	}
}

// Get response from cache (if it exists and is not expired), or determine a new
// response.
func getResponse(name, t string) (response uint8, fromCache bool) {
	// First check override
	// TODO: It might be better/faster to clear cache entries when adding
	// override?
	if checkOverride(name) {
		return reponseForward, false
	}

	cachekey := t + " " + name
	_cachelock.Lock()
	cache, haveCache := _cache[cachekey]
	_cachelock.Unlock()

	if haveCache && cache.expires > time.Now().Unix() {
		return cache.response, true
	}

	response = determineResponse(name, t)

	_cachelock.Lock()
	_cache[cachekey] = cacheT{
		expires:  time.Now().Unix() + int64(_config.CacheDNS),
		response: response,
	}
	_cachelock.Unlock()

	return response, false
}

func checkOverride(name string) bool {
	// Check override
	expires, haveOverride := _overrideHosts[name]

	if !haveOverride {
		labels := strings.Split(name, ".")
		c := ""
		l := len(labels)
		for i := 0; i < l; i++ {
			if c == "" {
				c = labels[l-i-1]
			} else {
				c = labels[l-i-1] + "." + c
			}

			expires, haveOverride = _overrideHosts[c]
			if haveOverride {
				break
			}
		}
	}

	// Make sure it's not expires
	if haveOverride {
		if time.Now().Unix() > expires {
			delete(_overrideHosts, name)
			haveOverride = false
		}
	}

	return haveOverride
}

// Determine what to do with the hostname name. Returns a RESPONSE_* constant.
func determineResponse(name, t string) uint8 {
	var doSpoof bool

	haveOverride := checkOverride(name)
	if haveOverride {
		doSpoof = false
	}

	if !haveOverride {
		// Hosts
		labels := strings.Split(name, ".")
		c := ""
		l := len(labels)
		for i := 0; i < l; i++ {
			if c == "" {
				c = labels[l-i-1]
			} else {
				c = labels[l-i-1] + "." + c
			}

			_, doSpoof = _hosts[c]
			if doSpoof {
				break
			}
		}

		// Regexps
		if !doSpoof {
			for _, r := range _regexps {
				if r.MatchString(name) {
					doSpoof = true
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
	if doSpoof && t == "AAAA" {
		return reponseEmty
	} else if doSpoof {
		return reponseSpoof
	} else {
		return reponseForward
	}
}

// Spoof DNS response by replying with the address of our HTTP server. This only
// does A records.
func spoof(name string, w dns.ResponseWriter, req *dns.Msg) {
	spec := fmt.Sprintf("%s. 1 IN A %s", name, _config.HTTPListen.Host)
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

	// Set cache to 0
	for i := range answer {
		answer[i].Header().Ttl = 0
	}

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
