package srvdns

import (
	"fmt"
	"testing"
)

func TestCache(t *testing.T) {
	cases := []struct {
		in          *CacheList
		expectedLen int
	}{
		{&CacheList{}, 0},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			outLen := tc.in.Len()
			if outLen != tc.expectedLen {
				t.Errorf("\nout:      %#v\nexpected: %#v\n", outLen, tc.expectedLen)
			}
		})
	}

}
