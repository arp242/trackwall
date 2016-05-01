// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Various helper functions used throughout the application.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	syscall "golang.org/x/sys/unix"
)

// Prefix a path with the chroot dir.
func chrootdir(path string) (path_with_chroot string) {
	return _config.chroot + "/" + path
}

func realpath(path string) (string, error) {
	path = filepath.Clean(path)
	// TODO: Errors out if dest doesn't exist
	//return filepath.EvalSymlinks()
	return path, nil
}

// Drop privileges
func drop_privs() {
	// TODO Don't do this on Linux systems for now.
	//
	// Calls to this are peppered throughout since on Linux different threads can
	// have a different uid/gid, and the syscall only sets it for the *current*
	// thread.
	// See: https://github.com/golang/go/issues/1435
	//
	// This is only an issue on Linux, not on other systems.
	//
	// This is really a quick stop-hap solution and we should do this better. One
	// way is to start a new process after the privileged initialisation and pass
	// the filenos to that, but that would require reworking quite a bit of the DNS
	// server bits in the dns package...
	if runtime.GOOS == "linux" {
		return
	}

	info("dropping privileges")

	err := syscall.Setresgid(_config.user.gid, _config.user.gid, _config.user.gid)
	fatal(err)
	err = syscall.Setresuid(_config.user.uid, _config.user.uid, _config.user.uid)
	fatal(err)

	// Double-check just to be sure.
	if syscall.Getuid() != _config.user.uid || syscall.Getgid() != _config.user.gid {
		fatal(fmt.Errorf("unable to drop privileges"))
	}
}

// Convert a human-readable duration to the number of seconds. A "duration" is
// simply a number with a suffix. Accepted suffixes are:
//   no suffix: seconds
//   s: seconds
//   m: minutes
//   h: hours
//   d: days
//   w: weeks
//   M: months (a month is 30.5 days)
//   y: years (a year is always 365 days)
func durationToSeconds(dur string) (int, error) {
	last := dur[len(dur)-1]

	// Last character is a number
	_, err := strconv.Atoi(string(last))
	if err == nil {
		return strconv.Atoi(dur)
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
	return i * fact, err
}

// Remove old cache items every 5 minutes.
func startCachePurger() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)

			_cachelock.Lock()
			i := 0
			for name, cache := range _cache {
				// Don't lock stuff too long
				if i > 1000 {
					break
				}

				if time.Now().Unix() > cache.expires {
					delete(_cache, name)
				}
				i += 1
			}
			_cachelock.Unlock()
		}
	}()
}

// Send a message. You should never call this yourself, use fatal(), warn(),
// info(), infoc(), or dbg().
func msg(prefix, msg interface{}, fillcolor string, fp io.Writer) {
	calldepth := 2
	_, file, line, ok := runtime.Caller(calldepth)
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

	fill := strings.Repeat(" ", 20-len(s))
	switch fillcolor {
	case "orange":
		fill = orangebg(fill)
	case "green":
		fill = greenbg(fill)
	case "red":
		fill = redbg(fill)
	}
	fmt.Fprintf(fp, "%s %s %v\n", s, fill, msg)
}

// Exit if err is non-nil
func fatal(err error) {
	if err == nil {
		return
	}

	msg("error", err, "red", os.Stderr)
	os.Exit(1)
}

// Warn if err is non-nil
func warn(err error) {
	if err == nil {
		return
	}

	msg("warn", err, "red", os.Stderr)
}

func info(m string) {
	if _verbose {
		msg("info", m, "", os.Stdout)
	}
}

func infoc(m, color string) {
	if _verbose {
		msg("info", m, color, os.Stdout)
	}
}

func dbg(m string) {
	if false {
		msg("debug", m, "", os.Stdout)
	}
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

// The MIT License (MIT)
//
// Copyright © 2016 Martin Tournoij
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// The software is provided "as is", without warranty of any kind, express or
// implied, including but not limited to the warranties of merchantability,
// fitness for a particular purpose and noninfringement. In no event shall the
// authors or copyright holders be liable for any claim, damages or other
// liability, whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or other dealings
// in the software.
