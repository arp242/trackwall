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

// "Compile" a surrogate into the config.hosts array. This uses a bit more memory,
// but saves a lot of regexp checks later.
func (s *ConfigT) compileSurrogate(reg string, sur string) {
	sur = strings.Replace(sur, "@@", "function(){}", -1)
	//info(fmt.Sprintf("compiling surrogate %s -> %s", reg, sur[:40]))

	c, err := regexp.Compile(reg)

	Surrogates.Lock()
	Surrogates.l = append(Surrogates.l, SurrogateEntry{c, sur})
	Surrogates.Unlock()

	msg.Fatal(err)

	found := 0
	Hosts.Lock()
	defer Hosts.Unlock()
	for host := range Hosts.m {
		if c.MatchString(host) {
			found++
			//info(fmt.Sprintf("  adding for %s", host))
			Hosts.m[host] = sur
		}
	}

	if found > 50 {
		msg.Warn(fmt.Errorf("the surrogate %s matches %d hosts. Are you sure this is correct?",
			reg, found))
	}
}
