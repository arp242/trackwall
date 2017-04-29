package srvdns

import (
	"io"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
)

// CacheList is the list of all caches entries.
type CacheList struct {
	sync.RWMutex
	m map[string]CacheEntry
}

// CacheEntry is a single cached entry
type CacheEntry struct {
	response uint8
	expires  int64
}

// Cache of spoofing actions.
//
// We don't cache the actual DNS responses − that's the resolver's job. We just
// cache the action taken. That's enough and saves some time in processing
// regexps and such
var Cache CacheList

func init() {
	Cache = CacheList{}
	Cache.Purge()
}

// Len returns the lenght of the map.
func (l *CacheList) Len() int {
	l.Lock()
	defer l.Unlock()
	return len(l.m)
}

// Get a single item.
func (l *CacheList) Get(k string) (CacheEntry, bool) {
	l.RLock()
	v, ok := l.m[k]
	l.RUnlock()
	return v, ok
}

// Store an item.
func (l *CacheList) Store(k string, entry CacheEntry) {
	l.Lock()
	l.m[k] = entry
	l.Unlock()
}

// Delete items.
func (l *CacheList) Delete(keys ...string) {
	l.Lock()
	defer l.Unlock()
	for _, k := range keys {
		delete(l.m, k)
	}
}

// Purge the entire cache
func (l *CacheList) Purge() {
	l.Lock()
	l.m = make(map[string]CacheEntry)
	l.Unlock()
}

// PurgeExpired removes old cache items.
func (l *CacheList) PurgeExpired(max int) {
	l.Lock()

	i := 0
	for name, cache := range l.m {
		// Don't lock stuff too long
		if i > max {
			break
		}

		if time.Now().Unix() > cache.expires {
			delete(l.m, name)
		}
		i++
	}
	l.Unlock()
}

// Dump all keys to the writer.
func (l *CacheList) Dump(w io.Writer) {
	l.RLock()
	defer l.RUnlock()

	scs := spew.ConfigState{Indent: "\t"}
	scs.Fdump(w, l.m)
}
