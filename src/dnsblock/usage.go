// Usage info
package main

import (
	"fmt"
	"os"
	"strings"
)

var _usage = map[string]string{
	"global_opts": `
Global options:
    -v, --verbose   Verbose output
    -h, --help      Show help
    -c, --config    Path to the configuration file

`,

	"global": `
Usage: %[1]s command [arguments]

Commands:
    help      Show this help
	version   Show version and exit
    server    Run as DNS/HTTP server
    status    Show server status
    host      Control host list
    regex     Control regexp list

%[2]s

Use %[1]s [command] -h for more help on a specific command.
`,

	"server": `
Usage: %[1]s server [arguments]

%[2]s
`,

	"status": `
Usage: %[1]s status [arguments]

%[2]s
`,

	"host": `
Usage: %[1]s host [arguments]

%[2]s
`,

	"regex": `
Usage: %[1]s regex [arguments]

%[2]s
`,
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
// TODO: Use or write something better
func getopt(args []string, shortopts string) (parsed map[string]string, err error) {
	// Always accept these
	shortopts += "hvc:"

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
