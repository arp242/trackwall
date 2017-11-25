package cfg

import (
	"fmt"
	"io"
	"regexp"
	"sync"
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

// Len returns the length of the list.
func (l *RegexpList) Len() int {
	l.Lock()
	defer l.Unlock()
	return len(l.l)
}

// Add regexps.
func (l *RegexpList) Add(regexps ...string) {
	l.Lock()
	defer l.Unlock()

	for _, re := range regexps {
		l.l = append(l.l, regexp.MustCompile(re))
	}
}

// Remove regexps.
func (l *RegexpList) Remove(regexps ...string) {
	l.Lock()
	defer l.Unlock()

	for _, re := range regexps {
		for i, r := range l.l {
			if r.String() == re {
				l.l = append(l.l[:i], l.l[i+1:]...)
				break
			}
		}
	}
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
