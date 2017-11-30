// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Package cfg contains the configuration and related global states.
package cfg

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"

	"arp242.net/sconfig"
	"arp242.net/trackwall/msg"
)

// AddrT is an IP or hostname
type AddrT struct {
	Host string
	Port int
	IPv6 bool
}

// Get it as a string: host:port
func (a *AddrT) String() string {
	if a.IPv6 {
		return fmt.Sprintf("[%v]:%v", a.Host, a.Port)
	}
	return fmt.Sprintf("%v:%v", a.Host, a.Port)
}

// Set it from a host:port string.
func (a *AddrT) set(addr string) {
	if addr[0] != '[' && strings.Count(addr, ":") > 1 {
		addr = fmt.Sprintf("[%v]:53", addr)
	} else if !strings.Contains(addr, ":") {
		addr += ":53"
	}

	if addr[0] == '[' {
		a.IPv6 = true
	}

	host, port, err := net.SplitHostPort(addr)
	msg.Fatal(err)
	a.Host = host
	a.Port, err = strconv.Atoi(port)
	msg.Fatal(err)
}

// UserT is a system user
type UserT struct {
	user.User

	// the user.User.{Uid,Gid} are strings, not ints :-/
	UID int
	GID int
}

// Set it from a username.
func (u *UserT) set(username string) {
	user, err := user.Lookup(username)
	msg.Fatal(err)
	u.User = *user

	u.UID, err = strconv.Atoi(user.Uid)
	msg.Fatal(err)

	u.GID, err = strconv.Atoi(user.Gid)
	msg.Fatal(err)
}

// Load a config file from path
func Load(path string) error {
	sconfig.RegisterType("*cfg.AddrT", sconfig.ValidateSingleValue(),
		func(v []string) (interface{}, error) {
			a := &AddrT{}
			a.set(v[0])
			return a, nil
		})
	sconfig.RegisterType("*cfg.UserT", sconfig.ValidateSingleValue(),
		func(v []string) (interface{}, error) {
			u := &UserT{}
			u.set(v[0])
			return u, nil
		})

	return sconfig.Parse(&Config, path, sconfig.Handlers{
		"CacheDNS": func(l []string) error {
			Config.CacheDNS, _ = msg.DurationToSeconds(l[0])
			return nil
		},
		"CacheHosts": func(l []string) error {
			Config.CacheHosts, _ = msg.DurationToSeconds(l[0])
			return nil
		},
		"Hostlists": func(l []string) error {
			for _, v := range l[1:] {
				Config.Hostlists = append(Config.Hostlists, []string{l[0], v})
			}
			return nil
		},
		"Unhostlists": func(l []string) error {
			for _, v := range l[1:] {
				Config.Unhostlists = append(Config.Unhostlists, []string{l[0], v})
			}
			return nil
		},
		"Regexplists": func(l []string) error {
			for _, v := range l[1:] {
				Config.Regexplists = append(Config.Regexplists, []string{l[0], v})
			}
			return nil
		},
		"Unregexplists": func(l []string) error {
			for _, v := range l[1:] {
				Config.Regexplists = append(Config.Regexplists, []string{l[0], v})
			}
			return nil
		},
		"Hosts": func(l []string) error {
			Config.Hosts = append(Config.Hosts, l...)
			return nil
		},
		"Unhosts": func(l []string) error {
			Config.Unhosts = append(Config.Unhosts, l...)
			return nil
		},
		"Regexps": func(l []string) error {
			Config.Regexps = append(Config.Regexps, l...)
			return nil
		},
		"Unregexps": func(l []string) error {
			Config.Unregexps = append(Config.Unregexps, l...)
			return nil
		},
		"Surrogates": func(l []string) error {
			Config.Surrogates = append(Config.Surrogates, []string{l[0], strings.Join(l[1:], " ")})
			return nil
		},
	})
}

// nolint: megacheck
func findResolver() (string, error) {
	fp, err := os.Open("/etc/resolv.conf")
	msg.Fatal(err)

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "nameserver") {
			continue
		}
		if strings.HasSuffix(line, Config.DNSListen.Host) {
			continue
		}

		return line[strings.LastIndex(line, " ")+1:] + ":53", nil
	}

	return "", fmt.Errorf("unable to find host in /etc/resolv.conf")
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
