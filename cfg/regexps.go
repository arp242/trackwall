package cfg

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"arp242.net/trackwall/msg"
)

// RegexpList is a list of all regexp blocks.
//
// Pre-compiling the surrogate scripts isn't possible here.
type RegexpList struct {
	sync.RWMutex
	l []*regexp.Regexp
}

var (
	// Regexps are all the loaded regexps.
	Regexps RegexpList
)

func init() {
	Regexps = RegexpList{}
	Regexps.Purge()
}

// Len returns the lenght of the list.
func (l *RegexpList) Len() int {
	l.Lock()
	defer l.Unlock()
	return len(l.l)
}

// Match the name against all the regexps.
func (l *RegexpList) Match(name string) bool {
	l.Lock()
	defer l.Unlock()

	for _, r := range l.l {
		if r.MatchString(name) {
			return true
		}
	}
	return false
}

// Dump all keys to the writer.
func (l *RegexpList) Dump(w io.Writer) {
	l.Lock()
	defer l.Unlock()
	for _, v := range l.l {
		fmt.Fprintf(w, fmt.Sprintf("%v\n", v))
	}
}

// Purge the entire list
func (l *RegexpList) Purge() {
	l.Lock()
	l.l = []*regexp.Regexp{}
	l.Unlock()
}

// Add regexp
func (s *ConfigT) addRegexp(v string) {
	c, err := regexp.Compile(v)
	msg.Fatal(err)
	Regexps.Lock()
	Regexps.l = append(Regexps.l, c)
	Regexps.Unlock()
}

// Remove regexp
func (s *ConfigT) removeRegexp(v string) {
	Regexps.Lock()
	defer Regexps.Unlock()
	for i, r := range Regexps.l {
		if r.String() == v {
			Regexps.l = append(Regexps.l[:i], Regexps.l[i+1:]...)
			return
		}
	}
}
