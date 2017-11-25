// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package cmd

import (
	"os"

	"arp242.net/trackwall/cfg"
	"github.com/spf13/cobra"
)

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Compile the host list",
	Long: `
Compile all the hosts (as added with hostlist, host, unhostlist, and unhost in
the configuration file) to one "compiled" file with duplicates and redundant
entries removed. trackwall doesn't do this automatically on startup since this
is a comparatively expensive operation.

You don't strictly need to do this, but it will make the program start up and
run a bit faster.

The result is written to /compiled-hosts in the chroot directory and is used
automatically if its mtime is not older than cache-hosts. If it's older
trackwall will show a warning and ignore the file.`,
	Run: func(cmd *cobra.Command, args []string) {
		compile()
	},
}

func init() {
	RootCmd.AddCommand(compileCmd)
}

func compile() {
	chroot()
	DropPrivs()

	_ = os.Remove("/cache/compiled")
	cfg.Config.ReadHosts()
	cfg.Config.Compile()
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
