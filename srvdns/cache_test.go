package srvdns

import (
	"fmt"
	"testing"

	"arp242.net/trackwall/tt"
)

func TestCache(t *testing.T) {
	cases := []struct {
		in          *CacheList
		expectedLen int
	}{
		{&CacheList{}, 0},
		{&CacheList{
			m: map[string]CacheEntry{
				"x": CacheEntry{},
			},
		}, 1},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			tt.Eq(t, "outLen", tc.expectedLen,
				tc.in.Len())
		})
	}

}
