package cfg

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"arp242.net/trackwall/msg"
)

// ConfigT holds the config
type ConfigT struct {
	ControlListen *AddrT
	DNSListen     *AddrT
	DNSForward    *AddrT
	HTTPListen    *AddrT
	HTTPSListen   *AddrT
	RootCert      string
	RootKey       string
	User          *UserT
	Chroot        string
	CacheHosts    int64
	CacheDNS      int64
	Color         bool

	// A list of the various sources
	Hostlists     [][]string
	Unhostlists   [][]string
	Regexplists   [][]string
	Unregexplists [][]string

	Hosts     []string
	Unhosts   []string
	Regexps   []string
	Unregexps []string

	Surrogates [][]string
}

var (
	// Config of the app
	Config ConfigT
)

// ChrootDir prefixes a path with the chroot dir.
func (s ConfigT) ChrootDir(path string) string {
	return filepath.Join(s.Chroot, path)
}

// ReadHosts the hosts information *after* starting the DNS server because we can add
// hosts from remote sources (and thus needs DNS)
func (s *ConfigT) ReadHosts() {
	stat, err := os.Stat("/cache/compiled")

	if err == nil {
		expires := stat.ModTime().Add(time.Duration(Config.CacheHosts) * time.Second)
		if time.Now().Unix() > expires.Unix() {
			msg.Warn(fmt.Errorf("the compiled list has expired, not using it"))
		} else {
			msg.Info("using the compiled list", Verbose)
			fp, err := os.Open("/cache/compiled")
			msg.Fatal(err)
			defer func() { _ = fp.Close() }()

			scanner := bufio.NewScanner(fp)
			for scanner.Scan() {
				s.addHost(scanner.Text())
			}
			return
		}
	}

	for _, v := range s.Hostlists {
		s.loadList(v[0], v[1], s.addHost)
	}
	for _, v := range s.Unhostlists {
		s.loadList(v[0], v[1], s.removeHost)
	}
	for _, v := range s.Regexplists {
		s.loadList(v[0], v[1], s.addRegexp)
	}
	for _, v := range s.Unregexplists {
		s.loadList(v[0], v[1], s.removeRegexp)
	}

	for _, v := range s.Hosts {
		s.addHost(v)
	}
	for _, v := range s.Unhosts {
		s.removeHost(v)
	}
	for _, v := range s.Regexps {
		s.addRegexp(v)
	}
	for _, v := range s.Unregexps {
		s.removeRegexp(v)
	}

	for _, v := range s.Surrogates {
		s.compileSurrogate(v[0], v[1])
	}
}

// Load a list and execute cb() on every item we find.
// TODO: Add option to restrict format (e.g. regexplist hosts ... shouldn't be
// allowed).
// TODO: Allow loading remote config files in the trackwall format (which only
// parses host, hostlist, etc. and *not* dns-listen and such).
func (s *ConfigT) loadList(format string, url string, cb func(line string)) {
	fp, err := s.loadCachedURL(url)
	msg.Fatal(err)
	defer func() { _ = fp.Close() }()
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
			msg.Fatal(fmt.Errorf("unknown format: %v", format))
		}

		cb(line)
	}
}

// Load URL with cache.
func (s *ConfigT) loadCachedURL(url string) (*os.File, error) {
	// Load from filesystem
	if strings.HasPrefix(url, "file://") {
		return os.Open(url[7:])
	}

	// TODO: Check error (e.g. perm. denied)
	err := os.MkdirAll("/cache/hosts", 0755)
	msg.Fatal(err)
	cachename := "/cache/hosts/" + regexp.MustCompile(`\W+`).ReplaceAllString(url, "-")

	stat, err := os.Stat(cachename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Check if cache expires
	if stat != nil {
		expires := stat.ModTime().Add(time.Duration(Config.CacheHosts) * time.Second)
		if time.Now().Unix() > expires.Unix() {
			stat = nil
			err := os.Remove(cachename)
			if err != nil {
				return nil, err
			}
		}
	}

	// Download
	if stat == nil {
		msg.Info("downloading "+url, Verbose)
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()

		fp, err := os.Create(cachename)
		if err != nil {
			return nil, err
		}

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		_, err = fp.Write(data)
		if err != nil {
			return nil, err
		}

		_ = fp.Close()
	}

	return os.Open(cachename)
}
