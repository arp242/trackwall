package srvdns

import (
	"testing"
	"time"

	"arp242.net/trackwall/tt"
)

func TestCacheList(t *testing.T) {
	l := &Cache

	// Len
	tt.Eq(t, "len", 0, l.Len())

	l.Store("x", CacheEntry{1, 2})
	tt.Eq(t, "len", 1, l.Len())

	// Get
	e, ok := l.Get("x")
	tt.Eq(t, "get", CacheEntry{1, 2}, e)
	tt.Eq(t, "get", ok, true)

	e, ok = l.Get("zxc")
	tt.Eq(t, "get", CacheEntry{}, e)
	tt.Eq(t, "get", ok, false)

	// Delete
	l.Delete("x")
	e, ok = l.Get("x")
	tt.Eq(t, "len", 0, l.Len())
	tt.Eq(t, "get", CacheEntry{}, e)
	tt.Eq(t, "get", ok, false)

	// Purge
	l.Store("current", CacheEntry{
		expires:  time.Now().Add(10 * time.Second).Unix(),
		response: 1,
	})
	l.Store("old", CacheEntry{
		expires:  time.Now().Add(-3600 * time.Second).Unix(),
		response: 1,
	})
	tt.Eq(t, "len", 2, l.Len())
	l.PurgeExpired(100)
	tt.Eq(t, "len", 1, l.Len())

	e, ok = l.Get("old")
	tt.Eq(t, "get", CacheEntry{}, e)
	tt.Eq(t, "get", ok, false)

	e, ok = l.Get("current")
	tt.Eq(t, "get", ok, true)
}
