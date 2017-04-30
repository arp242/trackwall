package cmdline

import (
	"fmt"
	"testing"

	"arp242.net/trackwall/tt"
)

func TestProcess(t *testing.T) {
	cases := []struct {
		inArgs []string
	}{
		{[]string{}},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			Process(tc.inArgs)
		})
	}
}

func TestUsage(t *testing.T) {
	cases := []struct {
		inName string
		inErr  string
	}{
		{"", ""},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			Usage(tc.inName, tc.inErr)
		})
	}
}

func Test_getopt(t *testing.T) {
	cases := []struct {
		inArgs []string
		inOpts string

		expectedOpts  map[string]string
		expectedWords []string
		expectedErr   error
	}{
		{
			[]string{}, "",
			map[string]string{}, *new([]string), nil,
		},
		{
			[]string{"hello"}, "",
			map[string]string{}, []string{"hello"}, nil,
		},
		{
			[]string{"hello", "world"}, "",
			map[string]string{}, []string{"hello", "world"}, nil,
		},
		{
			[]string{"-x"}, "x",
			map[string]string{"-x": ""}, *new([]string), nil,
		},
		{
			[]string{"-x", "test"}, "x",
			map[string]string{"-x": ""}, []string{"test"}, nil,
		},
		{
			[]string{"test", "-x"}, "x",
			map[string]string{"-x": ""}, []string{"test"}, nil,
		},
		{
			[]string{"--", "-x", "test"}, "x",
			map[string]string{}, []string{"-x", "test"}, nil,
		},
		{
			[]string{"-xy"}, "xyz:",
			map[string]string{"-x": "", "-y": ""}, *new([]string), nil,
		},
		{
			[]string{"-xyz", "asd"}, "xyz:",
			map[string]string{"-x": "", "-y": "", "-z": "asd"}, *new([]string), nil,
		},
		{
			[]string{"-z", "asd", "-x"}, "xyz:",
			map[string]string{"-x": "", "-z": "asd"}, *new([]string), nil,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			outOpts, outWords, outErr := getopt(tc.inArgs, tc.inOpts)
			tt.Eq(t, "outOpts", tc.expectedOpts, outOpts)
			tt.Eq(t, "outWords", tc.expectedWords, outWords)
			tt.Eq(t, "outErr", tc.expectedErr, outErr)
		})
	}
}
