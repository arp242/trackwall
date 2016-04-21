// DNS proxy which can spoof responses to block ads and malicious websites.
//
// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

const _blocked = `<html><head><title>dnsblock %[1]s</title></head><body>
<p>dnsblock blocked access to <code>%[1]s</code>. Unblock this domain for:</p>
<ul><li><a href="/$@_allow/10s/%[2]s">ten seconds</a></li>
<li><a href="/$@_allow/1h/%[2]s">an hour</a></li>
<li><a href="/$@_allow/1d/%[2]s">a day</a></li>
<li><a href="/$@_allow/10y/%[2]s">until restart</a></li></ul></body></html>`

type config_t struct {
	// As [addr, port]
	dns_listen   []string
	dns_forward  []string
	http_listen  []string
	https_listen []string

	https_cert string
	https_key  string

	user        user.User
	uid         int
	gid         int
	chroot      string
	cache_hosts int
}

var _config config_t

// The key is the hostname
// The (optional) value is a noop script to serve
var _hosts map[string]string

// Hosts to override; value is timestamp, once that's expired the entry will be
// removed from the list
var _override_hosts map[string]int64

var _spooftypes = map[uint16]string{
	dns.TypeA: "A",
	// TODO: Figure out the best way to handle AAAA records
	//dns.TypeAAAA: "AAAA",
	// TODO: does spoofing cnames work? Is it really required?
	//dns.TypeCNAME: "CNAME",
}

func main() {
	_override_hosts = make(map[string]int64)

	file := "config"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}
	sources, noops := parseConfig(file)

	// Setup chroot
	info(fmt.Sprintf("chrooting to %v", _config.chroot))
	syscall.Chroot(_config.chroot)
	// Make a fake /etc/resolv.conf so that go's resolver recognizes it
	err := os.MkdirAll("/etc", 0755)
	fatal(err)
	fp, err := os.Create("/etc/resolv.conf")
	defer fp.Close()
	fp.Write([]byte(fmt.Sprintf("nameserver %s", _config.dns_listen[0])))

	// Setup HTTP server
	go func() {
		err := http.ListenAndServe(joinAddr(_config.http_listen), &handleHttp{})
		fatal(err)
	}()
	go func() {
		err := http.ListenAndServeTLS(joinAddr(_config.https_listen),
			_config.https_cert, _config.https_key, &handleHttp{})
		fatal(err)
	}()

	// Setup DNS server
	dns.HandleFunc(".", handleDns)
	dns_udp := dns.Server{Addr: joinAddr(_config.dns_listen), Net: "udp"}
	defer dns_udp.Shutdown()
	go func() {
		err := dns_udp.ListenAndServe()
		fatal(err)
	}()
	dns_tcp := dns.Server{Addr: joinAddr(_config.dns_listen), Net: "tcp"}
	defer dns_tcp.Shutdown()
	go func() {
		err := dns_tcp.ListenAndServe()
		fatal(err)
	}()

	// Wait for all servers to start
	// TODO: This can be better − in fact, I'd prefer to open the sockets much
	// earlier and do other init stuff later so we can drop privileges
	time.Sleep(1 * time.Second)
	info("servers started")

	// Drop privileges
	drop_privs()

	// Read the hosts information *after* starting the DNS server because we can
	// add hosts from remote sources (and thus needs DNS)
	for _, v := range sources {
		addHostlist(v[0], v[1])
	}
	for _, v := range noops {
		compileNoop(v[0], v[1])
	}

	// Wait for SIGINT or SIGTERM
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
}

// Drop privileges
func drop_privs() {
	err := syscall.Setresgid(_config.gid, _config.gid, _config.gid)
	fatal(err)
	err = syscall.Setresuid(_config.uid, _config.uid, _config.uid)
	fatal(err)
}

func handleDns(w dns.ResponseWriter, req *dns.Msg) {
	// Make sure we're using the correct uid in the callback; this is sometimes
	// still 0 on the first few requests due to the Setresuid() being called
	// after the goroutines that start the servers...
	drop_privs()

	if len(req.Question) == 0 {
		dns.HandleFailed(w, req)
		return
	}

	name := strings.TrimRight(req.Question[0].Name, ".")
	_, has_host := _hosts[name]
	spooftype, has_spooftype := _spooftypes[req.Question[0].Qtype]

	// TODO: This doesn't work very well since browsers cache the shit out of
	// DNS; so we may want to use a HTTP proxy for this?
	expires, has_override := _override_hosts[name]
	if has_override {
		if time.Now().Unix() > expires {
			delete(_override_hosts, name)
		} else {
			has_spooftype = false
		}
	}

	// We only need to spoof A records
	if has_spooftype && has_host {
		spoof(name, spooftype, w, req)
	} else {
		forward(joinAddr(_config.dns_forward), w, req)
	}
}

// Spoof DNS response
func spoof(name string, t string, w dns.ResponseWriter, req *dns.Msg) {
	var spec string
	//if t == "AAAA" {
	//	spec = fmt.Sprintf("%s. 3600 IN %s ::1",
	//		name, t)
	//} else {
	//spec = fmt.Sprintf("%s. 3600 IN %s %s",
	spec = fmt.Sprintf("%s. 1 IN %s %s",
		name, t, _config.http_listen[0])
	//}

	info(fmt.Sprintf("spoof   %v", spec))
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

// Forward DNS request
func forward(addr string, w dns.ResponseWriter, req *dns.Msg) {
	transport := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		transport = "tcp"
	}
	info(fmt.Sprintf("forward %v %v over %v to %v",
		req.Question[0].Name, typeName(req.Question[0].Qtype), transport, addr))
	c := &dns.Client{Net: transport}
	resp, _, err := c.Exchange(req, addr)
	if err != nil {
		dns.HandleFailed(w, req)
		return
	}
	w.WriteMsg(resp)
}

func typeName(t uint16) string {
	// TODO: See if we can do this more dynamically
	m := map[uint16]string{
		dns.TypeNone:       "None",
		dns.TypeA:          "A",
		dns.TypeNS:         "NS",
		dns.TypeMD:         "MD",
		dns.TypeMF:         "MF",
		dns.TypeCNAME:      "CNAME",
		dns.TypeSOA:        "SOA",
		dns.TypeMB:         "MB",
		dns.TypeMG:         "MG",
		dns.TypeMR:         "MR",
		dns.TypeNULL:       "NULL",
		dns.TypeWKS:        "WKS",
		dns.TypePTR:        "PTR",
		dns.TypeHINFO:      "HINFO",
		dns.TypeMINFO:      "MINFO",
		dns.TypeMX:         "MX",
		dns.TypeTXT:        "TXT",
		dns.TypeRP:         "RP",
		dns.TypeAFSDB:      "AFSDB",
		dns.TypeX25:        "X25",
		dns.TypeISDN:       "ISDN",
		dns.TypeRT:         "RT",
		dns.TypeNSAPPTR:    "NSAPPTR",
		dns.TypeSIG:        "SIG",
		dns.TypeKEY:        "KEY",
		dns.TypePX:         "PX",
		dns.TypeGPOS:       "GPOS",
		dns.TypeAAAA:       "AAAA",
		dns.TypeLOC:        "LOC",
		dns.TypeNXT:        "NXT",
		dns.TypeEID:        "EID",
		dns.TypeNIMLOC:     "NIMLOC",
		dns.TypeSRV:        "SRV",
		dns.TypeATMA:       "ATMA",
		dns.TypeNAPTR:      "NAPTR",
		dns.TypeKX:         "KX",
		dns.TypeCERT:       "CERT",
		dns.TypeDNAME:      "DNAME",
		dns.TypeOPT:        "OPT",
		dns.TypeDS:         "DS",
		dns.TypeSSHFP:      "SSHFP",
		dns.TypeIPSECKEY:   "IPSECKEY",
		dns.TypeRRSIG:      "RRSIG",
		dns.TypeNSEC:       "NSEC",
		dns.TypeDNSKEY:     "DNSKEY",
		dns.TypeDHCID:      "DHCID",
		dns.TypeNSEC3:      "NSEC3",
		dns.TypeNSEC3PARAM: "NSEC3PARAM",
		dns.TypeTLSA:       "TLSA",
		dns.TypeHIP:        "HIP",
		dns.TypeNINFO:      "NINFO",
		dns.TypeRKEY:       "RKEY",
		dns.TypeTALINK:     "TALINK",
		dns.TypeCDS:        "CDS",
		dns.TypeCDNSKEY:    "CDNSKEY",
		dns.TypeOPENPGPKEY: "OPENPGPKEY",
		dns.TypeSPF:        "SPF",
		dns.TypeUINFO:      "UINFO",
		dns.TypeUID:        "UID",
		dns.TypeGID:        "GID",
		dns.TypeUNSPEC:     "UNSPEC",
		dns.TypeNID:        "NID",
		dns.TypeL32:        "L32",
		dns.TypeL64:        "L64",
		dns.TypeLP:         "LP",
		dns.TypeEUI48:      "EUI48",
		dns.TypeEUI64:      "EUI64",
		dns.TypeURI:        "URI",
		dns.TypeCAA:        "CAA",
		dns.TypeTKEY:       "TKEY",
		dns.TypeTSIG:       "TSIG",
		dns.TypeIXFR:       "IXFR",
		dns.TypeAXFR:       "AXFR",
		dns.TypeMAILB:      "MAILB",
		dns.TypeMAILA:      "MAILA",
		dns.TypeANY:        "ANY",
		dns.TypeTA:         "TA",
		dns.TypeDLV:        "DLV",
		dns.TypeReserved:   "Reserved",
	}
	v, _ := m[t]
	return v
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

			_override_hosts[host] = time.Now().Add(time.Duration(secs)*time.Second).Unix()
			w.Header().Set("Location", "/" + strings.Join(params[2:], "/"))
			w.WriteHeader(http.StatusSeeOther) // TODO: Is this the best 30x header?
		// $@_list/{config,hosts,override}
		} else if strings.HasPrefix(url, "$@_list") {
			param := strings.Split(url, "/")[1]
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
			}
		} else {
		}
	} else {
		noop, exists := _hosts[host]

		// This should never happen
		if !exists {
			warn(fmt.Errorf("host not in _hosts?!"))
		}

		// TODO: Do something sane with the Content-Type header
		if noop != "" {
			fmt.Fprintf(w, noop)
		} else {
			fmt.Fprintf(w, fmt.Sprintf(_blocked, host, url))
		}
	}
}

// Parse the config file
func parseConfig(file string) (hostlists, noops [][]string) {
	info(fmt.Sprintf("reading configuration from %v", file))
	fp, err := os.Open(file)
	fatal(err)
	defer fp.Close()

	_hosts = make(map[string]string)
	scanner := bufio.NewScanner(fp)
	//var noops [][]string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || line[0] == '#' {
			continue
		}

		for {
			if line[len(line)-1] == '\\' {
				scanner.Scan()
				line = line[:len(line)-1] + strings.TrimSpace(scanner.Text())
			} else {
				break
			}
		}

		splitline := strings.Split(line, " ")
		switch splitline[0] {
		case "dns-listen":
			_config.dns_listen = strings.Split(splitline[1], ":")
		case "dns-forward":
			_config.dns_forward = strings.Split(splitline[1], ":")
		case "http-listen":
			_config.http_listen = strings.Split(splitline[1], ":")
		case "https-listen":
			_config.https_listen = strings.Split(splitline[1], ":")
		case "https-cert":
			_config.https_cert = splitline[1]
		case "https-key":
			_config.https_key = splitline[1]
		case "user":
			user, err := user.Lookup(splitline[1])
			fatal(err)
			_config.user = *user
			_config.uid, err = strconv.Atoi(user.Uid)
			fatal(err)
			_config.gid, err = strconv.Atoi(user.Gid)
			fatal(err)
		case "chroot":
			_config.chroot, err = filepath.EvalSymlinks(path.Clean(splitline[1]))
			fatal(err)
		case "cache-hosts":
			_config.cache_hosts, err = durationToSeconds(splitline[1])
			fatal(err)
		case "hostlist":
			hostlists = append(hostlists, []string{splitline[1], splitline[2]})
		case "noop":
			noops = append(noops, []string{splitline[1], strings.Join(splitline[2:], " ")})
		default:
			fatal(fmt.Errorf("unknown config key: %v\n", splitline[0]))
		}
	}

	return hostlists, noops
}

func joinAddr(addr []string) string {
	return strings.Join(addr, ":")
}

// no suffix: seconds
// s: seconds
// m: minutes
// h: hours
// d: days
// w: weeks
// M: months (a month is 30.5 days)
// y: years (a "year" is always 365 days)
func durationToSeconds(dur string) (int, error) {
	last := dur[len(dur)-1]
	_, err := strconv.Atoi(string(last))
	if err == nil {
		return strconv.Atoi(dur)
	}

	var fact int
	switch last {
	case 's':
		fact = 1
	case 'm':
		fact = 60
	case 'h':
		fact = 3600
	case 'd':
		fact = 86400
	case 'w':
		fact = 604800
	case 'M':
		fact = 2635200
	case 'y':
		fact = 31536000
	default:
		return 0, fmt.Errorf("durationToSeconds: unable to parse %v", dur)
	}

	i, err := strconv.Atoi(dur[:len(dur)-1])
	return i * fact, err
}

// "Compile" the noops into the config.hosts array. This uses a bit more memory,
// but saves a lot of regexp checks later.
func compileNoop(reg string, noop string) {
	info(fmt.Sprintf("compiling noop %s", reg))

	noop = strings.Replace(noop, "@@", "function(){}", -1)

	c, err := regexp.Compile(reg)
	fatal(err)

	found := 0
	for host, _ := range _hosts {
		if c.MatchString(host) {
			found += 1
			info(fmt.Sprintf("adding noop for %s -> %s…", host, noop[:40]))
			_hosts[host] = noop
		}
	}

	if found > 50 {
		warn(fmt.Errorf("the noop %s matches %s hosts. Are you sure this is correct?",
			reg, found))
	}
}

func hashString(str string) string {
	h := sha256.Sum256([]byte(str))
	return hex.EncodeToString(h[:])
}

// Add hosts
func addHostlist(format string, url string) {
	var scanner *bufio.Scanner

	// Load from filesystem
	if strings.HasPrefix(url, "file://") {
		fp, err := os.Open(url[7:])
		fatal(err)
		defer fp.Close()
		scanner = bufio.NewScanner(fp)
		// Load from network
	} else {
		os.MkdirAll("/cache", 0755)

		cachename := "/cache/" + hashString(url)
		stat, err := os.Stat(cachename)
		if err != nil && !os.IsNotExist(err) {
			fatal(err)
		}

		if stat != nil {
			expires := stat.ModTime().Add(time.Duration(_config.cache_hosts) * time.Second)
			if time.Now().Unix() > expires.Unix() {
				stat = nil
				os.Remove(cachename)
			}
		}

		if stat == nil {
			resp, err := http.Get(url)
			fatal(err)
			defer resp.Body.Close()

			fp, err := os.Create(cachename)
			fatal(err)
			data, err := ioutil.ReadAll(resp.Body)
			fp.Write(data)
			fp.Close()
		}

		fp, err := os.Open(cachename)
		fatal(err)
		defer fp.Close()
		scanner = bufio.NewScanner(fp)
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if format == "hosts" {
			if line[0] == '#' {
				continue
			}
			line = strings.TrimSpace(strings.Join(strings.Split(line, " ")[1:], " "))
		} else if format == "plain" {
			// Nothing needed
		} else {
			fatal(fmt.Errorf("unknown format: %v\n", format))
		}

		_hosts[line] = ""
	}
}

// Exit if err is non-nil
func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Warn if err is non-nil
func warn(err error) {
	if err != nil {
		log.Print(err)
	}
}

func info(msg string) {
	//log.Print(msg)
	fmt.Println(msg)
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
