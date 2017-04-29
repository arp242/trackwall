package cfg

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"arp242.net/trackwall/msg"
)

// HostList is a static hosts added with hostlist/host. The key is the
// hostname, the (optional) value is a surrogate script to serve.
type HostList struct {
	sync.RWMutex
	m map[string]string
}

var (
	// Hosts are all the loaded hosts.
	Hosts HostList
)

func init() {
	Hosts = HostList{}
	Hosts.Purge()
}

// Get a single item.
func (l *HostList) Get(k string) (string, bool) {
	l.RLock()
	v, ok := l.m[k]
	l.RUnlock()
	return v, ok
}

// Len returns the lenght of the map.
func (l *HostList) Len() int {
	l.Lock()
	defer l.Unlock()
	return len(l.m)
}

// Dump all keys to the writer.
func (l *HostList) Dump(w io.Writer) {
	l.RLock()
	defer l.RUnlock()

	for k, v := range l.m {
		if v != "" {
			fmt.Fprintf(w, fmt.Sprintf("%v  # %v\n", k, v))
		} else {
			fmt.Fprintf(w, fmt.Sprintf("%v\n", k))
		}
	}
}

// Add host to _hosts
func (s *ConfigT) addHost(name string) {
	// Remove www.
	if strings.HasPrefix(name, "www.") {
		name = strings.Replace(name, "www.", "", 1)
	}

	// TODO: For some reason this happens sometimes. Find the source and fix
	// properly.
	if name == "" {
		return
	}

	// We already got this
	Hosts.Lock()
	defer Hosts.Unlock()
	if _, has := Hosts.m[name]; has {
		return
	}
	Hosts.m[name] = ""
}

// Remove host from _hosts
func (s *ConfigT) removeHost(v string) {
	Hosts.Lock()
	delete(Hosts.m, v)
	Hosts.Unlock()
}

// Purge the entire list
func (l *HostList) Purge() {
	l.Lock()
	l.m = make(map[string]string)
	l.Unlock()
}

// Compile all the sources in one file, saves some memory and makes lookups a
// bit faster
func (s *ConfigT) Compile() {
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
