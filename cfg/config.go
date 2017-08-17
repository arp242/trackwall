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

// ConfigT holds the configuration.
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
	Verbose       int

	// A list of the various sources; this only contains the hosts defined with
	// the "host" keyword in the config.
	Hostlists     [][]string
	Unhostlists   [][]string
	Regexplists   [][]string
	Unregexplists [][]string
	Hosts         []string
	Unhosts       []string
	Regexps       []string
	Unregexps     []string
	Surrogates    [][]string
}

// Config of the application.
var Config ConfigT

// ChrootDir prefixes a path with the chroot dir.
func (c ConfigT) ChrootDir(path string) string {
	return filepath.Join(c.Chroot, path)
}

// ReadHosts the hosts information in to the various variables (Hosts, Regexps,
// etc.)
func (c *ConfigT) ReadHosts() {
	c.ReadHostsLists()

	Hosts.Add(c.Hosts...)
	Hosts.Remove(c.Unhosts...)
	Regexps.Add(c.Regexps...)
	Regexps.Remove(c.Unregexps...)
	Surrogates.Add(c.Surrogates...)
}

// ReadHostsLists reads the hosts lists.
func (c *ConfigT) ReadHostsLists() {
	// Try to use cached file
	if stat, err := os.Stat("/cache/compiled"); err == nil {
		expires := stat.ModTime().Add(time.Duration(Config.CacheHosts) * time.Second)
		if expires.Unix() > time.Now().Unix() {
			c.readHostsCache()
			return
		}

		msg.Warn(fmt.Errorf("the compiled list has expired, not using it"))
	}

	c.loadList(Hosts.Add, c.Hostlists...)
	c.loadList(Hosts.Remove, c.Unhostlists...)
	c.loadList(Regexps.Add, c.Regexplists...)
	c.loadList(Regexps.Remove, c.Unregexplists...)
}

func (c *ConfigT) readHostsCache() {
	msg.Info("reading compiled list from /cache/compiled", Config.Verbose)
	fp, err := os.Open("/cache/compiled")
	msg.Fatal(err)
	defer func() { _ = fp.Close() }()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		Hosts.Add(scanner.Text())
	}
}

// Load a list and execute cb() on every item we find.
// TODO: Add option to restrict format (e.g. regexplist hosts ... shouldn't be
// allowed).
// TODO: Allow loading remote config files in the trackwall format (which only
// parses host, hostlist, etc. and *not* dns-listen and such).
func (c *ConfigT) loadList(cb func(line ...string), lists ...[]string) {
	for _, list := range lists {
		format := list[0]
		url := list[1]

		fp, err := c.loadCachedURL(url)
		msg.Fatal(err)
		defer func() { _ = fp.Close() }()
		scanner := bufio.NewScanner(fp)

		for scanner.Scan() {
			line := c.readLine(scanner, format)
			if line != "" {
				cb(line)
			}
		}
	}
}

func (c *ConfigT) readLine(scanner *bufio.Scanner, format string) string {
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return ""
	}

	switch format {
	case "plain":
		// Nothing needed

	// /etc/hosts format
	case "hosts":
		if line[0] == '#' {
			return ""
		}

		// Remove everything before the first space and after the first #
		line = strings.Join(strings.Split(line, " ")[1:], " ")
		line = strings.Split(line, "#")[0]
		line = strings.TrimSpace(line)

		// Some sites also add this to the hosts file they offer.
		if line == "localhost" || line == "localhost.localdomain" || line == "broadcasthost" || line == "local" {
			return ""
		}

	default:
		msg.Fatal(fmt.Errorf("unknown format: %v", format))
	}

	return line
}

// Load URL with cache.
func (c *ConfigT) loadCachedURL(url string) (*os.File, error) {
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

	// Check if cache expired
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
		msg.Info("downloading "+url, Config.Verbose)
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

// Compile all the sources in one file, saves some memory and makes lookups a
// bit faster
func (c *ConfigT) Compile() {
	newHosts := make(map[string]string)

	Hosts.Lock()
	defer Hosts.Unlock()

outer:
	for name := range Hosts.m {
		labels := strings.Split(name, ".")

		// This catches adding "s8.addthis.com" while "addthis.com" is in the list
		c := ""
		l := len(labels)
		for i := 0; i < l; i++ {
			if c == "" {
				c = labels[l-i-1]
			} else {
				c = labels[l-i-1] + "." + c
			}

			_, have := newHosts[c]
			if have {
				continue outer
			}
		}

		// This catches adding "addthis.com" while "s7.addthis.com" is in the list;
		// in which case we want to remove the former.
		for host := range newHosts {
			if strings.HasSuffix(host, name) {
				delete(newHosts, name)
			}
		}

		newHosts[name] = ""
	}

	fp, err := os.Create("/cache/compiled")
	msg.Fatal(err)
	defer func() { _ = fp.Close() }()
	for k := range newHosts {
		_, err = fp.WriteString(fmt.Sprintf("%v\n", k))
		msg.Fatal(err)
	}

	fmt.Printf("Compiled %v hosts to %v entries\n", len(Hosts.m), len(newHosts))
}
