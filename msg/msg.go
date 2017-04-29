// Package msg provides some tools for printing nice messages to the terminal.
//
// The location of the call is automatically prepended with all the calls.
package msg

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

var (
	stdout = os.Stdout
	stderr = os.Stderr
)

// Fatal prints the error and exits if it's non-nil.
func Fatal(err error) {
	if err == nil {
		return
	}

	msg("error", err, "red", stderr)
	os.Exit(1)
}

// Warn if err is non-nil
func Warn(err error) {
	if err == nil {
		return
	}
	msg("warn", err, "red", stderr)
}

// Info for informational messages.
func Info(m string, verbose bool) {
	if verbose {
		msg("info", m, "", stdout)
	}
}

// Infoc are informational messages in colour.
func Infoc(m, color string, verbose bool) {
	if verbose {
		msg("info", m, color, stdout)
	}
}

// Debug messages.
func Debug(m string) {
	if false {
		msg("debug", m, "", stdout)
	}
}

// DurationToSeconds converts a human-readable duration to the number of
// seconds.
// A "duration" is simply a number with a suffix. Accepted suffixes are:
//   no suffix: seconds
//   s: seconds
//   m: minutes
//   h: hours
//   d: days
//   w: weeks
//   M: months (a month is 30.5 days)
//   y: years (a year is always 365 days)
func DurationToSeconds(dur string) (int64, error) {
	last := dur[len(dur)-1]

	// Last character is a number
	_, err := strconv.Atoi(string(last))
	if err == nil {
		x, err := strconv.Atoi(dur)
		return int64(x), err
	}

	var fact int
	switch last {
	case 's':
		fact = 1
	case 'm':
		fact = 60
	case 'h':
		fact = 3600
	case 'd':
		fact = 86400
	case 'w':
		fact = 604800
	case 'M':
		fact = 2635200
	case 'y':
		fact = 31536000
	default:
		return 0, fmt.Errorf("durationToSeconds: unable to parse %v", dur)
	}

	i, err := strconv.Atoi(dur[:len(dur)-1])
	return int64(i * fact), err
}

func msg(prefix, msg interface{}, fillColor string, fp io.Writer) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short

	s := fmt.Sprintf("%s %v:%v", prefix, file, line)

	fill := strings.Repeat(" ", 24-len(s))
	switch fillColor {
	case "orange":
		fill = orangebg(fill)
	case "green":
		fill = greenbg(fill)
	case "red":
		fill = redbg(fill)
	}
	fmt.Fprintf(fp, "%s %s %v\n", s, fill, msg)
}

func greenbg(m string) string {
	return fmt.Sprintf("[48;5;154m%s[0m", m)
}

func orangebg(m string) string {
	return fmt.Sprintf("[48;5;221m%s[0m", m)
}

func redbg(m string) string {
	return fmt.Sprintf("[48;5;9m%s[0m", m)
}
