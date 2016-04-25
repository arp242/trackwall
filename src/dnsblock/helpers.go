// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// Various helper functions used throughout the application.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	syscall "golang.org/x/sys/unix"
)

func realpath(path string) (string, error) {
	path = filepath.Clean(path)
	// TODO: Errors out if dest doesn't exist
	//return filepath.EvalSymlinks()
	return path, nil
}

// Drop privileges
func drop_privs() {
	err := syscall.Setresgid(_config.user.gid, _config.user.gid, _config.user.gid)
	fatal(err)
	err = syscall.Setresuid(_config.user.uid, _config.user.uid, _config.user.uid)
	fatal(err)
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

func hashString(str string) string {
	h := sha256.Sum256([]byte(str))
	return hex.EncodeToString(h[:])
}

// Exit if err is non-nil
func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// Warn if err is non-nil
func warn(err error) {
	if err != nil {
		log.Print(err)
	}
}

func info(msg string) {
	fmt.Println(msg)
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
