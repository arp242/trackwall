package cfg

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"arp242.net/trackwall/msg"
)

// SurrogateList is the list of surrogate scripts to use.
type SurrogateList struct {
	sync.RWMutex
	l []SurrogateEntry
}

// SurrogateEntry will serve the script if the regexp matches.
type SurrogateEntry struct {
	*regexp.Regexp
	script string
}

var (
	// Surrogates are all the surrogate scripts.
	Surrogates SurrogateList
)

func init() {
	Surrogates = SurrogateList{}
	Surrogates.Purge()
}

// Find a surrogate.
func (l *SurrogateList) Find(host string) (script string, success bool) {
	// Exact match! Hurray! This is fastest.
	sur, exists := Hosts.Get(host)
	if exists && sur != "" {
		return sur, true
	}

	// Slower check if a regex matches the domain
	return l.match(host)
}

// match the host against all the surrogates.
func (l *SurrogateList) match(host string) (script string, gotMatch bool) {
	l.Lock()
	defer l.Unlock()
	for _, sur := range l.l {
		if sur.MatchString(host) {
			return sur.script, true
		}
	}

	return "", false
}

// Purge the entire list
func (l *SurrogateList) Purge() {
	l.Lock()
	l.l = []SurrogateEntry{}
	l.Unlock()
}

// Add new surrogates. The first list entry is the host regexp, the second the
// script.
func (l *SurrogateList) Add(scripts ...[]string) {
	l.Lock()
	defer l.Unlock()

	for _, v := range scripts {
		reg := v[0]
		sur := v[1]
		sur = strings.Replace(sur, "@@", "function(){}", -1)

		re := regexp.MustCompile(reg)
		Surrogates.l = append(Surrogates.l, SurrogateEntry{re, sur})

		// Add to the hosts; bit more memory/expensive now, but saves a lot of
		// regexp checks later on.
		found := 0
		for host := range Hosts.m {
			if re.MatchString(host) {
				found++
				Hosts.SetScript(host, sur)
			}
		}

		if found > 50 {
			msg.Warn(fmt.Errorf("the surrogate %s matches %d hosts", reg, found))
		}
	}
}
