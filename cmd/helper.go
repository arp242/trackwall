// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"
	"arp242.net/trackwall/srvctl"
	"arp242.net/trackwall/srvhttp"
)

func sendCmd(cmd *cobra.Command, args []string) {
	send := cmd.Use
	if cmd.HasParent() {
		send = cmd.Parent().Use + " " + cmd.Use
	}
	send += " " + strings.Join(args, " ")
	srvctl.Write(send)
}

// Setup chroot() from the information in cfg.Config
func chroot() {
	msg.Info(fmt.Sprintf("chrooting to %v", cfg.Config.Chroot), cfg.Config.Verbose)

	// Make sure the chroot dir exists with the correct permissions and such
	_, err := os.Stat(cfg.Config.Chroot)
	if os.IsNotExist(err) {
		msg.Warn(fmt.Errorf("chroot dir %s doesn't exist, attempting to create", cfg.Config.Chroot))
		msg.Fatal(os.MkdirAll(cfg.Config.Chroot, 0755))
		msg.Fatal(os.Chown(cfg.Config.Chroot, cfg.Config.User.UID, cfg.Config.User.GID))
	}

	// TODO: We do this *before* the chroot since on OpenBSD it needs access to
	// /dev/urandom, which we don't have in the chroot (and I'd rather not add
	// this as a dependency).
	// This should be fixed in Go 1.7 by using getentropy() (see #13785, #14572)
	if _, err := os.Stat(cfg.Config.ChrootDir(cfg.Config.RootKey)); os.IsNotExist(err) {
		err := srvhttp.MakeRootKey()
		if err != nil {
			msg.Fatal(err)
		}
	}
	if _, err := os.Stat(cfg.Config.ChrootDir(cfg.Config.RootCert)); os.IsNotExist(err) {
		srvhttp.MakeRootCert()
	}

	msg.Fatal(os.Chdir(cfg.Config.Chroot))
	err = syscall.Chroot(cfg.Config.Chroot)
	if err != nil {
		msg.Fatal(fmt.Errorf("unable to chroot to %v: %v", cfg.Config.Chroot, err.Error()))
	}

	// Setup /etc/resolv.conf in the chroot for Go's resolver
	err = os.MkdirAll("/etc", 0755)
	msg.Fatal(err)
	fp, err := os.Create("/etc/resolv.conf")
	msg.Fatal(err)
	_, err = fp.Write([]byte(fmt.Sprintf("nameserver %s", cfg.Config.DNSListen.Host)))
	msg.Fatal(err)
	err = fp.Close()
	msg.Fatal(err)

	// Make sure the rootCA files exist and are not world-readable.
	keyfile := func(path string) string {
		st, err := os.Stat(path)
		msg.Fatal(err)

		if st.Mode().Perm().String() != "-rw-------" {
			msg.Warn(fmt.Errorf("insecure permissions for %s, attempting to fix", path))
			msg.Fatal(os.Chmod(path, os.FileMode(0600)))
		}

		err = os.Chown(path, cfg.Config.User.UID, cfg.Config.User.GID)
		msg.Fatal(err)
		return path
	}

	cfg.Config.RootKey = keyfile(cfg.Config.RootKey)
	cfg.Config.RootCert = keyfile(cfg.Config.RootCert)
}

// DropPrivs drops to an unpriviliged user.
func DropPrivs() {
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
	//
	// setuidgid(8) should work
	if runtime.GOOS == "linux" {
		return
	}

	msg.Info("dropping privileges", cfg.Config.Verbose)

	err := syscall.Setresgid(cfg.Config.User.GID, cfg.Config.User.GID, cfg.Config.User.GID)
	msg.Fatal(err)
	err = syscall.Setresuid(cfg.Config.User.UID, cfg.Config.User.UID, cfg.Config.User.UID)
	msg.Fatal(err)

	// Double-check just to be sure.
	if syscall.Getuid() != cfg.Config.User.UID || syscall.Getgid() != cfg.Config.User.GID {
		msg.Fatal(fmt.Errorf("unable to drop privileges"))
	}
}

// The MIT License (MIT)
//
// Copyright © 2016-2017 Martin Tournoij
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
