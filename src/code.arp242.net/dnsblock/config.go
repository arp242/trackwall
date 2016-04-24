// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Parse the configuration file.
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Most of config (except for the sources).
type config_t struct {
	control_listen addr_t
	dns_listen     addr_t
	dns_forward    addr_t
	http_listen    addr_t
	https_listen   addr_t
	root_cert     string
	root_key      string
	user           user_t
	chroot         string
	cache_hosts    int

	sources sources_t

	// Lock the config file when loading/reloading. None of the global _*
	// variables (_config, _hosts, etc.) should be accessed when this is true
	// since not all data may be properly loaded.
	locked bool
}

// Parse the config file
func (c *config_t) parse(file string) {
	info(fmt.Sprintf("reading configuration from %v", file))
	c.locked = true

	fp, err := os.Open(file)
	fatal(err)
	defer fp.Close()

	//c.sources = sources_t{}
	scanner := bufio.NewScanner(fp)
	lines := []string{}
	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		is_indented := len(line) > 0 && unicode.IsSpace(rune(line[0]))
		line = strings.TrimSpace(line)

		if line == "" || line[0] == '#' {
			continue
		}

		if is_indented {
			lines[i-1] += " " + line
		} else {
			lines = append(lines, line)
			i += 1
		}
	}

	one := func(a []string) string {
		if len(a) < 2 || len(a) > 2 {
			fatal(fmt.Errorf("the %s option takes exactly one value (%v given).", a[0], len(a)-1))
		}

		return a[1]
	}

	three := func(a []string) []string {
		if len(a) < 3 {
			fatal(fmt.Errorf("the %s option takes at least three values (%v given)", a[0], len(a)-1))
		}
		return a[2:]
	}

	many := func(a []string) []string {
		if len(a) < 2 {
			fatal(fmt.Errorf("the %s option takes at least one values (%v given)", a[0], len(a)-1))
		}
		return a[1:]
	}

	for _, line := range lines {
		splitline := strings.Split(line, " ")
		switch splitline[0] {
		// Options
		case "control-listen":
			c.control_listen.set(one(splitline))
		case "dns-listen":
			c.dns_listen.set(one(splitline))
		case "dns-forward":
			c.dns_forward.set(one(splitline))
		case "http-listen":
			c.http_listen.set(one(splitline))
		case "https-listen":
			c.https_listen.set(one(splitline))
		case "root-cert":
			c.root_cert = one(splitline)
		case "root-key":
			c.root_key = one(splitline)
		case "user":
			c.user.set(one(splitline))
		case "chroot":
			c.chroot, err = realpath(one(splitline))
			fatal(err)
		case "cache-hosts":
			c.cache_hosts, err = durationToSeconds(one(splitline))
			fatal(err)
		// File sources
		case "hostlist":
			for _, v := range three(splitline) {
				c.sources.hostlists = append(c.sources.hostlists, []string{splitline[1], v})
			}
		case "unhostlist":
			for _, v := range three(splitline) {
				c.sources.unhostlists = append(c.sources.unhostlists, []string{splitline[1], v})
			}
		case "regexplist":
			for _, v := range three(splitline) {
				c.sources.regexplists = append(c.sources.regexplists, []string{splitline[1], v})
			}
		case "unregexplist":
			for _, v := range three(splitline) {
				c.sources.unregexplists = append(c.sources.unregexplists, []string{splitline[1], v})
			}
		// Sources in config
		case "host":
			for _, v := range many(splitline) {
				c.sources.hosts = append(c.sources.hosts, v)
			}
		case "unhost":
			for _, v := range many(splitline) {
				c.sources.unhosts = append(c.sources.unhosts, v)
			}
		case "regexp":
			many(splitline)
			for _, v := range many(splitline) {
				c.sources.regexps = append(c.sources.regexps, v)
			}
		case "unregexp":
			many(splitline)
			for _, v := range many(splitline) {
				c.sources.unregexps = append(c.sources.unregexps, v)
			}
		case "surrogate":
			many(splitline)
			c.sources.surrogates = append(c.sources.surrogates, []string{splitline[1], strings.Join(splitline[2:], " ")})
		// Other
		case "source":
			c.parse(one(splitline))
		default:
			fatal(fmt.Errorf("unknown config key: %v\n", splitline[0]))
		}
	}

	// Make sure the rootCA files exist and are not world-readable; we need some
	// other config values for that so do this after parsing.
	keyfile := func(f string) string {
		path := fmt.Sprintf("%s/%s", c.chroot, f)
		st, err := os.Stat(path)
		if os.IsNotExist(err) {
			fatal(err)
		}
		if st.Mode().Perm().String() != "-rw-------" {
			fatal(fmt.Errorf("the permission of %v must be exactly -rw------- (or 0600); currently %s", path, st.Mode().Perm()))
		}

		err = os.Chown(path, c.user.uid, c.user.gid)
		fatal(err)

		return f
	}
	
	c.root_key = keyfile(c.root_key)
	c.root_cert = keyfile(c.root_cert)
}

// An IP or hostname
type addr_t struct {
	host string
	port int
}

// Get it as a string: host:port
func (a addr_t) String() string {
	return fmt.Sprintf("%v:%v", a.host, a.port)
}

// Set it from a host:port string.
func (a *addr_t) set(addr string) {
	host, port, err := net.SplitHostPort(addr)
	fatal(err)
	a.host = host
	a.port, err = strconv.Atoi(port)
	fatal(err)
}

// A system user
type user_t struct {
	user.User

	// the user.User.{Uid,Gid} are strings, not ints :-/
	uid int
	gid int
}

// Set it from a username.
func (u *user_t) set(username string) {
	user, err := user.Lookup(username)
	fatal(err)
	u.User = *user

	u.uid, err = strconv.Atoi(user.Uid)
	fatal(err)

	u.gid, err = strconv.Atoi(user.Gid)
	fatal(err)
}

// A list of the various sources
type sources_t struct {
	hostlists     [][]string
	unhostlists   [][]string
	regexplists   [][]string
	unregexplists [][]string

	hosts     []string
	unhosts   []string
	regexps   []string
	unregexps []string

	surrogates [][]string
}

// Read the hosts information *after* starting the DNS server because we can add
// hosts from remote sources (and thus needs DNS)
// TODO: properly process all of it
func (sources sources_t) read() {
	for _, v := range sources.hostlists {
		sources.loadList(v[0], v[1], sources.addHost)
	}
	for _, v := range sources.unhostlists {
		sources.loadList(v[0], v[1], sources.removeHost)
	}
	for _, v := range sources.regexplists {
		sources.loadList(v[0], v[1], sources.addRegexp)
	}
	for _, v := range sources.unregexplists {
		sources.loadList(v[0], v[1], sources.removeRegexp)
	}

	for _, v := range sources.hosts {
		sources.addHost(v)
	}
	for _, v := range sources.unhosts {
		sources.removeHost(v)
	}
	for _, v := range sources.regexps {
		sources.addRegexp(v)
	}
	for _, v := range sources.unregexps {
		sources.removeRegexp(v)
	}

	for _, v := range sources.surrogates {
		sources.compileSurrogate(v[0], v[1])
	}
}

// Add host to _hosts
func (s sources_t) addHost(v string) {
	_hosts[v] = ""
}

// Remove host from _hosts
func (s sources_t) removeHost(v string) {
	delete(_hosts, v)
}

// Add regexp to _regexpx
func (s sources_t) addRegexp(v string) {
	c, err := regexp.Compile(v)
	fatal(err)
	_regexps = append(_regexps, c)
}

// Remove regexp to _regexpx
func (s sources_t) removeRegexp(v string) {
	for _, r := range _regexps {
		if r.String() == v {
			// TODO
		}
	}
}

// Load a list and execute cb() on every item we find.
// TODO: Add option to restrict format (i.e. regexplist hosts file:///...
// shouldn't be allowed).
func (s sources_t) loadList(format string, url string, cb func(line string)) {
	fp, err := s.loadCachedURL(url)
	fatal(err)
	defer fp.Close()
	scanner := bufio.NewScanner(fp)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		if format == "hosts" {
			if line[0] == '#' {
				continue
			}
			// Remove everything before the first space and after the first #
			line = strings.Join(strings.Split(line, " ")[1:], " ")
			line = strings.Split(line, "#")[0]
			line = strings.TrimSpace(line)

			// Some sites also add this to the hosts file they offer, which is
			// not wanted for us
			if line == "localhost" || line == "localhost.localdomain" || line == "broadcasthost" || line == "local" {
				continue
			}
		} else if format == "plain" {
			// Nothing needed
		} else {
			fatal(fmt.Errorf("unknown format: %v\n", format))
		}

		cb(line)
	}
}

// Load URL with cache
func (s sources_t) loadCachedURL(url string) (*os.File, error) {
	// Load from filesystem
	if strings.HasPrefix(url, "file://") {
		return os.Open(url[7:])
	}

	// Load from network
	os.MkdirAll("/cache/hosts", 0755)

	// TODO: Use a more readable filename
	cachename := "/cache/hosts/" + hashString(url)
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

	return os.Open(cachename)
}

// "Compile" a surrogate into the config.hosts array. This uses a bit more memory,
// but saves a lot of regexp checks later.
func (s sources_t) compileSurrogate(reg string, sur string) {
	sur = strings.Replace(sur, "@@", "function(){}", -1)
	info(fmt.Sprintf("compiling surrogate %s -> %s", reg, sur[:40]))

	c, err := regexp.Compile(reg)
	fatal(err)

	found := 0
	for host, _ := range _hosts {
		if c.MatchString(host) {
			found += 1
			info(fmt.Sprintf("  adding for %s", host))
			_hosts[host] = sur
		}
	}

	if found > 50 {
		warn(fmt.Errorf("the surrogate %s matches %s hosts. Are you sure this is correct?",
			reg, found))
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
