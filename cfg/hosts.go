package cfg

import (
	"fmt"
	"io"
	"strings"
	"sync"
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

// Add hosts.
func (l *HostList) Add(hosts ...string) {
	l.Lock()
	defer l.Unlock()

	for _, host := range hosts {
		if strings.HasPrefix(host, "www.") {
			host = strings.Replace(host, "www.", "", 1)
		}

		// We already got this
		if _, has := l.m[host]; has {
			return
		}
		l.m[host] = ""
	}
}

// SetScript sets the surrogate script for a host.
func (l *HostList) SetScript(host, script string) {
	l.Lock()
	defer l.Unlock()
	l.m[host] = script
}

// Remove hosts.
func (l *HostList) Remove(hosts ...string) {
	l.Lock()
	defer l.Unlock()
	for _, host := range hosts {
		delete(l.m, host)
	}
}

// Len returns the length of the map.
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

// Purge the entire list
func (l *HostList) Purge() {
	l.Lock()
	l.m = make(map[string]string)
	l.Unlock()
}
