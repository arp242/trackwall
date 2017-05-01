// Package tt implements some basic testing helpers.
package tt

import (
	"reflect"
	"testing"
)

var isTests = false

// Eq calls Errorf if out and expected are not equal.
func Eq(t *testing.T, name string, expected, out interface{}) {
	if !reflect.DeepEqual(out, expected) {
		if isTests {
			panic("not eq")
		}
		t.Errorf("wrong value for %v\nout:      %#v\nexpected: %#v\n",
			name, out, expected)
	}
}

// Err calls Fatalf is err is non-nil.
func Err(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Panic runs the function and calls Errorf if there is *no* panic.
func Panic(t *testing.T, f func()) {
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("expected panic")
		}
	}()
	f()
}
