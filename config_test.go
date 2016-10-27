package main

import (
	"os/user"
	"testing"
)

func TestAddrT(t *testing.T) {
	tests := map[string]AddrT{
		"127.0.0.1":        {"127.0.0.1", 53, false},
		"127.0.0.1:5353":   {"127.0.0.1", 5353, false},
		"127.0.0.1:539999": {"127.0.0.1", 539999, false},

		"arp242.net":      {"arp242.net", 53, false},
		"arp242.net:5353": {"arp242.net", 5353, false},

		"[::1]:80":                                     {"::1", 80, true},
		"2a02:c7d:91f:f200:5361:4f9d:4b92:644b":        {"2a02:c7d:91f:f200:5361:4f9d:4b92:644b", 53, true},
		"[2a02:c7d:91f:f200:5361:4f9d:4b92:644b]:5353": {"2a02:c7d:91f:f200:5361:4f9d:4b92:644b", 5353, true},
	}
	for test, expected := range tests {
		result := AddrT{}
		result.set(test)
		if result != expected {
			t.Errorf("%#v != %#v\n", result, expected)
		}
		if result.String() != expected.String() {
			t.Errorf("%s != %s\n", result.String(), expected.String())
		}
	}

	// TODO: difficult to test since we call fatal() and just quit :-/
	//errors := []string{
	//	"not a:port",
	//}
	//for _, test := range errors {
	//	result := AddrT{}
	//	result.set(test)
	//}
}

func TestUserT(t *testing.T) {
	tests := map[string]UserT{
		"root": {user.User{Uid: "0", Gid: "0", Username: "root", Name: "root", HomeDir: "/root"}, 0, 0},
	}

	for test, expected := range tests {
		result := UserT{}
		result.set(test)
		if result != expected {
			t.Errorf("%#v != %#v\n", result, expected)
		}
	}
}
