package cfg

import (
	"io"
	"sync"

	"github.com/davecgh/go-spew/spew"
)

// OverrideList are all the hosts the user told us to override.
//
// The value is expiry timestamp.
type OverrideList struct {
	sync.RWMutex
	m map[string]int64
}

// Override these hosts and regexps.
var Override OverrideList

func init() {
	Override = OverrideList{}
	Override.Purge()
}

// Get a single item.
func (l *OverrideList) Get(k string) (int64, bool) {
	l.RLock()
	v, ok := l.m[k]
	l.RUnlock()
	return v, ok
}

// Store an item.
func (l *OverrideList) Store(k string, expire int64) {
	l.Lock()
	l.m[k] = expire
	l.Unlock()
}

// Delete items.
func (l *OverrideList) Delete(keys ...string) {
	l.Lock()
	defer l.Unlock()
	for _, k := range keys {
		delete(l.m, k)
	}
}

// Purge the entire list
func (l *OverrideList) Purge() {
	l.Lock()
	l.m = make(map[string]int64)
	l.Unlock()
}

// Dump all keys to the writer.
func (l *OverrideList) Dump(w io.Writer) {
	l.RLock()
	defer l.RUnlock()

	scs := spew.ConfigState{Indent: "\t"}
	scs.Fdump(w, l.m)
}
