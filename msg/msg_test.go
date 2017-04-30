package msg

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"arp242.net/trackwall/tt"
)

func TestWarn(t *testing.T) {
	cases := []struct {
		in       error
		expected string
	}{
		{nil, ""},
		{errors.New("test warning"),
			"warn msg_test.go:29 \x1b[48;5;9m     \x1b[0m test warning\n"},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			stdoutBuf := &bytes.Buffer{}
			stderrBuf := &bytes.Buffer{}
			stdout = stdoutBuf
			stderr = stderrBuf

			Warn(tc.in)

			if stdoutBuf.String() != "" {
				t.Errorf("stdout wasn't empty: %v", stdoutBuf.String())
			}
			out := stderrBuf.String()
			if out != tc.expected {
				t.Errorf("\nout:      %#v\nexpected: %#v\n", out, tc.expected)
			}
		})
	}
}

func TestDurationToSeconds(t *testing.T) {
	cases := []struct {
		in          string
		expected    int64
		expectedErr error
	}{
		{"123", 123, nil},
		{"123s", 123, nil},
		{"1h", 3600, nil},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			out, outErr := DurationToSeconds(tc.in)
			tt.Eq(t, "out", tc.expected, out)
			tt.Eq(t, "out", tc.expectedErr, outErr)
		})
	}
}
