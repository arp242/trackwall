// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Package srvdns handles all the DNS stuff.
package srvdns

import (
	"fmt"
	"net"
	"strings"
	"time"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"

	"github.com/miekg/dns"
)

const (
	reponseForward = 1
	reponseSpoof   = 2
	reponseEmpty   = 3
)

// From config
var (
	dnsForward string
	dnsCache   int64
	httpAddr   string
	verbose    int
)

// Serve DNS requests.
//
// TODO: Splitting out the binding of the socket and starting a server is not
// easy with the dns API, so we don't for now.
func Serve(addr string, fwd string, cache int64, http string, v int) (*dns.Server, *dns.Server) {
	dnsForward = fwd
	dnsCache = cache
	httpAddr = http
	verbose = v
	dns.HandleFunc(".", handleDNS)

	dnsUDP := &dns.Server{Addr: addr, Net: "udp"}
	go func() {
		err := dnsUDP.ListenAndServe()
		msg.Fatal(err)
	}()

	dnsTCP := &dns.Server{Addr: addr, Net: "tcp"}
	go func() {
		err := dnsTCP.ListenAndServe()
		msg.Fatal(err)
	}()

	// Remove old cache items every 5 minutes.
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			Cache.PurgeExpired(1000)
		}
	}()

	return dnsUDP, dnsTCP
}

// Handle a DNS request: either forward or spoof it.
func handleDNS(w dns.ResponseWriter, req *dns.Msg) {
	// No or invalid question section? Just bail out.
	if len(req.Question) == 0 {
		dns.HandleFailed(w, req)
		return
	}

	// We only need to spoof A and AAAA records; we can forward everything else.
	t := dns.TypeToString[req.Question[0].Qtype]
	if t != "A" && t != "AAAA" {
		forward(dnsForward, w, req)
		return
	}

	name := strings.TrimRight(req.Question[0].Name, ".")
	response, fromCache := getResponse(name, t)

	switch response {
	case reponseForward:
		if !fromCache {
			msg.Infoc(fmt.Sprintf("forward  %v", name), "green", verbose)
		}
		forward(dnsForward, w, req)
	case reponseSpoof:
		if !fromCache {
			msg.Infoc(fmt.Sprintf("spoof    %v", name), "orange", verbose)
		}
		spoof(name, w, req)
	case reponseEmpty:
		spoofEmpty(w, req)
	}
}

// Get response from cache (if it exists and is not expired), or determine a new
// response.
func getResponse(name, t string) (response uint8, fromCache bool) {
	if checkOverride(name) {
		return reponseForward, false
	}

	cachekey := t + " " + name

	cache, haveCache := Cache.Get(cachekey)
	if haveCache && cache.expires > time.Now().Unix() {
		return cache.response, true
	}

	response = determineResponse(name, t)
	Cache.Store(cachekey, CacheEntry{
		expires:  time.Now().Unix() + dnsCache,
		response: response,
	})

	return response, false
}

// Determine what to do with the hostname name.
//
// Returns a response* constant.
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

			_, doSpoof = cfg.Hosts.Get(c)
			if doSpoof {
				break
			}
		}

		// Regexps
		if !doSpoof {
			doSpoof = cfg.Regexps.Match(name)
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
		return reponseEmpty
	} else if doSpoof {
		return reponseSpoof
	} else {
		return reponseForward
	}
}

func checkOverride(name string) bool {
	expires, haveOverride := cfg.Override.Get(name)

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

			expires, haveOverride = cfg.Override.Get(c)
			if haveOverride {
				break
			}
		}
	}

	// Make sure it's not expired
	if haveOverride {
		if time.Now().Unix() > expires {
			cfg.Override.Delete(name)
			haveOverride = false
		}
	}
	return haveOverride
}

// Spoof DNS response by replying with the address of our HTTP server.
// This only does A records.
func spoof(name string, w dns.ResponseWriter, req *dns.Msg) {
	spec := fmt.Sprintf("%s. 1 IN A %s", name, httpAddr)
	rr, err := dns.NewRR(spec)
	msg.Fatal(err)

	sendSpoof([]dns.RR{rr}, w, req)
}

// Spoof DNS response by replying with an empty answer section.
func spoofEmpty(w dns.ResponseWriter, req *dns.Msg) {
	sendSpoof([]dns.RR{}, w, req)
}

// Make a message with the answer and write it to the client
func sendSpoof(answer []dns.RR, w dns.ResponseWriter, req *dns.Msg) {
	var spoof dns.Msg
	spoof.MsgHdr.Id = req.MsgHdr.Id
	spoof.MsgHdr.Response = true
	spoof.MsgHdr.RecursionDesired = true
	spoof.MsgHdr.RecursionAvailable = true
	spoof.Question = req.Question

	// Set cache to 0
	for i := range answer {
		answer[i].Header().Ttl = 0
	}

	spoof.Answer = answer
	spoof.Ns = []dns.RR{}
	spoof.Extra = []dns.RR{}

	err := w.WriteMsg(&spoof)
	if err != nil {
		msg.Warn(fmt.Errorf("unable to spoof DNS request for %v: %v",
			req.Question[0], err))
	}
}

// Forward the DNS request req to the server at addr and send the answer back to
// the client.
func forward(addr string, w dns.ResponseWriter, req *dns.Msg) {
	transport := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		transport = "tcp"
	}

	c := &dns.Client{Net: transport}
	resp, _, err := c.Exchange(req, addr)
	if err != nil {
		dns.HandleFailed(w, req)
		msg.Warn(fmt.Errorf("unable to forward DNS request for %v to %v: %v",
			req.Question[0], addr, err))
		return
	}

	err = w.WriteMsg(resp)
	if err != nil {
		msg.Warn(fmt.Errorf("unable to write DNS request for %v to %v: %v",
			req.Question[0], addr, err))
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
