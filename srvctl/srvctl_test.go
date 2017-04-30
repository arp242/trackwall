package srvctl

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestReadCommand(t *testing.T) {
	cases := []struct {
		in           string
		expected     []string
		expectedHTTP bool
		expectedErr  error
	}{
		{"command\n", []string{"command"}, false, nil},
		{"command test\n", []string{"command", "test"}, false, nil},
		{"command test\r\n", []string{"command", "test"}, false, nil},
		{"command  test\n", []string{"command", "", "test"}, false, nil},

		{"GET /command HTTP/1.1\r\n", []string{"command"}, true, nil},
		{"GET /command/test HTTP/1.1\r\n", []string{"command", "test"}, true, nil},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("%v", tc.in), func(t *testing.T) {
			out, outHTTP, outErr := readCommand(strings.NewReader(tc.in))
			if outErr != nil {
				t.Fatal(outErr)
			}
			if !reflect.DeepEqual(tc.expected, out) {
				t.Errorf("\nout:      %#v\nexpected: %#v\n", out, tc.expected)
			}

			if outHTTP != tc.expectedHTTP {
				t.Errorf("\nout:      %#v\nexpected: %#v\n", outHTTP, tc.expectedHTTP)
			}
		})
	}
}
