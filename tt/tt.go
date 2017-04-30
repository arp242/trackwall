// Package tt implements some basic testing tools.
package tt

import (
	"reflect"
	"testing"
)

// Eq calls Errorf if out and expected are not equal.
func Eq(t *testing.T, name string, expected, out interface{}) {
	if !reflect.DeepEqual(out, expected) {
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
