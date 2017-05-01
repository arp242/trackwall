// Package cmdline processes the commandline arguments.
package cmdline

import (
	"fmt"
	"os"
	"strings"

	"arp242.net/sconfig"
)

var _usage = map[string]string{
	// Global opts
	"global_opts": `
Global arguments:
    -v        Verbose output; use twice for debug output
    -h        Show help
    -f path   Path to the configuration file

`,

	// Global
	"global": `
Usage: %[1]s command [arguments]

%[2]s

Commands:
    help      Show this help
    version   Show version and exit
    server    Run as DNS/HTTP server
    compile   Compile the host list

Controlling a running trackwall instance:
    status    Show server status
        summary   Show a brief summary
        config    Show the configuration values
        cache     Show the cache
        hosts     Show hosts (may be a lot of output)
        regexps   Show regexps
        override  Show override table
    cache     Control cache
        flush     Flush all cache
    override  Control override
        flush     Flush all overrides
    host      Control host list
        add       Add new hosts
        rm        Remove hosts
    regex     Control regexp list
        add       Add new regexp
        rm        Remove regexp

Use '%[1]s [command] -h' or '%[1]s help command' for more detailed help.
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

%[2]s

Start the DNS and HTTP(S) server. This is the main operation of the program.
Almost all options are controlled through the configuration file.

Note that trackwall cannot run as a "daemon", and we assume that the system
provides some mechanism to cope with this (many do, such as daemontools, runit,
systemd, upstart, etc).
For systems that don't provide this, you'll need to use a wrapper.

`,
	// Compile
	"compile": `
Usage: %[1]s compile [arguments]

%[2]s

Compile all the hosts (as added with hostlist, host, unhostlist, and unhost in
the configuration file) to one "compiled" file with duplicates and redundant
entries removed. trackwall doesn't do this automatically on startup since this
is a comparatively expensive operation.

You don't strictly need to do this, but it will make the program start up and
run a bit faster.

The result is written to /compiled-hosts in the chroot directory and is used
automatically if its mtime is not older than cache-hosts. If it's older trackwall
will show a warning and ignore the file.
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

Get status of running Trackwall instance.
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
}

// Process commandline arguments
func Process(args []string) (
	words []string,
	config string,
	verbose int64,
	err error,
) {
	opts, words, err := getopt(args, "")
	if err != nil {
		return nil, "", 0, err
	}

	showHelp := false
	for opt, arg := range opts {
		switch opt {
		case "-h":
			showHelp = true
		case "-f":
			config = arg
		case "-v":
			verbose++
		}
	}

	if config == "" {
		config = sconfig.FindConfig("trackwall/config")
	}

	if showHelp {
		if len(words) == 0 {
			Usage("global", "")
		} else {
			Usage(words[0], "")
		}
		os.Exit(0)
	}

	return words, config, verbose, nil
}

// Usage prints out the help info.
func Usage(name, err string) {
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

// Args is the list of argument the user gave (e.g. os.Args), shortopts is the
// definition of options.
func getopt(args []string, shortopts string) (opts map[string]string, words []string, err error) {
	shortopts += "hvf:"

	// First split args into separate options so that "-hv" and "-hf myfile"
	// work.
	newargs := []string{}
	for _, arg := range args {
		// Command
		if !strings.HasPrefix(arg, "-") {
			newargs = append(newargs, arg)
			continue
		}

		for _, char := range arg[1:] {
			newargs = append(newargs, fmt.Sprintf("-%v", string(char)))
		}
	}

	args = newargs

	// Now parse the options
	opts = make(map[string]string)
	stopParsing := false
	skipNext := false
	for argi, arg := range args {
		if arg == "--" {
			stopParsing = true
			continue
		}

		if skipNext {
			skipNext = false
			continue
		}

		if !strings.HasPrefix(arg, "-") || stopParsing {
			words = append(words, arg)
			continue
		}

		for chari, char := range shortopts {
			if char != rune(arg[1]) {
				continue
			}

			if string(shortopts[chari+1]) == ":" {
				if len(args) <= argi+1 {
					return opts, words, fmt.Errorf("the option %s requires an argument", arg)
				}
				opts[arg] = args[argi+1]
				skipNext = true
			} else {
				opts[arg] = ""
			}
		}
	}

	return opts, words, nil
}
