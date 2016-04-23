// Parse configuration.
//
// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.
package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Most of config (except for the sources).
type config_t struct {
	// As [addr, port]
	control_listen []string
	dns_listen     []string
	dns_forward    []string
	http_listen    []string
	https_listen   []string

	https_cert string
	https_key  string

	user        user.User
	uid         int
	gid         int
	chroot      string
	cache_hosts int
}

// A list of the various sources
type sources_t struct {
	hostlists     [][]string
	unhostlists   [][]string
	hosts         []string
	unhosts       []string
	regexplists   [][]string
	regexps       []string
	unregexplists [][]string
	unregexps     []string
	surrogates    [][]string
}

// Add hosts
func loadHostlist(format string, url string, unhost bool) {
	fp, err := loadHostURL(url)
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

		if unhost {
			delete(_hosts, line)
		} else {
			_hosts[line] = ""
		}
	}
}

func addHostlist(format string, url string) {
	loadHostlist(format, url, false)
}

func addUnhostlist(format string, url string) {
	loadHostlist(format, url, true)
}

// Load URL with cache
func loadHostURL(url string) (*os.File, error) {
	// Load from filesystem
	if strings.HasPrefix(url, "file://") {
		return os.Open(url[7:])
	}

	// Load from network
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

	return os.Open(cachename)
}

// "Compile" a surrogate into the config.hosts array. This uses a bit more memory,
// but saves a lot of regexp checks later.
func compileSurrogate(reg string, sur string) {
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

// Parse the config file
func parseConfig(file string) sources_t {
	info(fmt.Sprintf("reading configuration from %v", file))
	fp, err := os.Open(file)
	fatal(err)
	defer fp.Close()

	sources := sources_t{}
	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		// Process \ as line continuation
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
		// Options
		case "control-listen":
			_config.control_listen = strings.Split(splitline[1], ":")
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
			_config.chroot, err = realpath(splitline[1])
			fatal(err)
		case "cache-hosts":
			_config.cache_hosts, err = durationToSeconds(splitline[1])
			fatal(err)
		// Sources
		case "hostlist":
			sources.hostlists = append(sources.hostlists, []string{splitline[1], splitline[2]})
		case "host":
			sources.hosts = append(sources.hosts, splitline[1])
		case "unhost":
			sources.unhosts = append(sources.unhosts, splitline[1])
		case "unhostlist":
			sources.unhostlists = append(sources.unhostlists, []string{splitline[1], splitline[2]})
		case "regexplist":
			sources.regexplists = append(sources.regexplists, []string{splitline[1], splitline[2]})
		case "regexp":
			sources.regexps = append(sources.regexps, splitline[1])
		case "unregexp":
			sources.unregexps = append(sources.unregexps, splitline[1])
		case "unregexplist":
			sources.unregexplists = append(sources.unregexplists, []string{splitline[1], splitline[2]})
		case "surrogate":
			sources.surrogates = append(sources.surrogates, []string{splitline[1], strings.Join(splitline[2:], " ")})
		default:
			fatal(fmt.Errorf("unknown config key: %v\n", splitline[0]))
		}
	}

	return sources
}

// Read the hosts information *after* starting the DNS server because we can add
// hosts from remote sources (and thus needs DNS)
func readSources(sources sources_t) {
	for _, v := range sources.hostlists {
		addHostlist(v[0], v[1])
	}
	for _, v := range sources.hosts {
		_hosts[v] = ""
	}
	for _, v := range sources.unhostlists {
		addUnhostlist(v[0], v[1])
	}
	for _, v := range sources.unhosts {
		delete(_hosts, v)
	}

	for _, v := range sources.regexps {
		c, err := regexp.Compile(v)
		fatal(err)
		_regexps = append(_regexps, c)
	}

	for _, v := range sources.surrogates {
		compileSurrogate(v[0], v[1])
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
