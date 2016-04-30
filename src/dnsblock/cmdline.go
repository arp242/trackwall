// Process commandline arguments
// TODO: Much of this file is ad-hoc and not exactly great. I wasn't able to get
// the flags package to do what I wanted. It works well enough for now but I
// should revisit this before a 1.0 release.
package main

import (
	"fmt"
	"os"
	"strings"
)

var _usage = map[string]string{
	// Global opts
	"global_opts": `
Global options:
    -v        Verbose output
    -h        Show help
    -f        Path to the configuration file

`,

	// Global
	"global": `
Usage: %[1]s command [arguments]

Commands:
    help      Show this help
    version   Show version and exit
    server    Run as DNS/HTTP server
    compile   Compile the host list
    status    Show server status
    cache     Control cache
    override  Control override
    host      Control host list
    regex     Control regexp list
    log       Get log information

%[2]s

Use %[1]s [command] -h for more help on a specific command.
`,

	// Help
	"help": `
Usage: %[1]s help [command]

Show help. You can optionally add a command name to show the help for that
command (this is the same as running %[1]s [command] -h).

`,

	// Version
	"version": `
Usage: %[1]s version

Show program version and exit with code 0.
`,

	// Server
	"server": `
Usage: %[1]s server [arguments]

Start the DNS and HTTP(S) server. This is the main operation of the program.
Almost all options are controlled through the configuration file.

Note that dnsblock cannot run as a "daemon", and we assume that the system
provides some mechanism to cope with this (many do, such as daemontools, runit,
systemd, upstart, etc).
For systems that don't provide this, you'll need to use a wrapper.

%[2]s

`,
	// Compile
	"compile": `
Usage: %[1]s compile [arguments]

Compile all the hosts (as added with hostlist, host, unhostlist, and unhost in
the configuration file) to one "compiled" file with duplicates and redundant
entries removed. dnsblock doesn't do this automatically on startup since this is
a comparativly expensive operation.

In the default configuration, the amount of hosts are reduced from 45,608
entries (875Kb) to 36,798 entries (678Kb). If we include some overhead for every
item, then this seems to save a total of about 480Kb. It also makes some lookups
slightly faster.

The result is written to /compiled-hosts in the chroot directory and is used
automatically if its mtime is not older than cache-hosts. If it's older dnsblock
will show a warning and ignore the file.

%[2]s
`,

	// Status
	"status": `
Usage: %[1]s status [arguments] command

%[2]s

Commands:
    summary   Show a brief summary
    config    Show the configuration values
    cache     Show the cache
    hosts     Show hosts (may be a lot of output)
    regexps   Show regexps
    override  Show override table
`,

	// Cache
	"cache": `
Usage: %[1]s regex [arguments] command

%[2]s

Commands:
    flush     Flush all cache
`,

	// Override
	"override": `
Usage: %[1]s regex [arguments] command

%[2]s

Commands:
    flush     Flush all overrides
`,

	// Host
	"host": `
Usage: %[1]s host [arguments] command

%[2]s

Commands:
    add [host1 host2 ...]  Add new hosts
    rm [host1 host2 ...]   Remove hosts
`,

	// Regex
	"regex": `
Usage: %[1]s regex [arguments] command

%[2]s

Commands:
    add [regexp1 regexp2 ...]  Add new regexp
    rm [regexp1 regexp2 ...]   Remove regexp
`,

	// Log
	"log": `
Usage: %[1]s log [arguments]

Get all log messages

%[2]s
`,
}

// Process commandline arguments
func cmdline() (command string) {
	// No command, print help
	if len(os.Args) < 2 {
		usage("global", "")
		return
	}

	args, err := getopt(os.Args[2:], "")
	fatal(err)

	show_help := false
	config := "/etc/dnsblock/config"

	for opt, arg := range args {
		switch opt {
		case "-h":
			show_help = true
		case "-f":
			config = arg
		case "-v":
			_verbose = true
		}
	}

	if show_help {
		usage(os.Args[1], "")
		os.Exit(0)
	}

	_config.parse(config, true)

	return os.Args[1]
}

// Print usage info
func usage(name, err string) {
	out := os.Stdout
	if err != "" {
		fmt.Fprintf(out, "Error: %s\n\n", err)
		out = os.Stderr
	}

	fmt.Fprintf(out, strings.TrimSpace(_usage[name])+"\n",
		os.Args[0], strings.TrimSpace(_usage["global_opts"]))

	if err != "" {
		os.Exit(1)
	}
}

// A rudimentary getopt()...
func getopt(args []string, shortopts string) (parsed map[string]string, err error) {
	// Always accept these
	shortopts += "hvf:"

	parsed = make(map[string]string)
	for argi, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			continue
		}

		for chari, char := range shortopts {
			if char != rune(arg[1]) {
				continue
			}

			if string(shortopts[chari+1]) == ":" {
				if len(args) <= argi+1 {
					return parsed, fmt.Errorf("the option %s requires an argument", arg)
				} else {
					parsed[arg] = args[argi+1]
				}
			} else {
				parsed[arg] = ""
			}
		}
	}

	return parsed, nil
}
