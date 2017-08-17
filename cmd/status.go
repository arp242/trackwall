// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package cmd

import "github.com/spf13/cobra"

var (
	statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show server status",
		Long:  `Get status of running trackwall instance.`,
	}
	statusSummaryCmd = &cobra.Command{
		Use:   "summary",
		Short: "Show a brief summary",
		Run:   sendCmd,
	}
	statusConfigCmd = &cobra.Command{
		Use:   "config",
		Short: "Show the configuration values",
		Run:   sendCmd,
	}
	statusCacheCmd = &cobra.Command{
		Use:   "cache",
		Short: "Show the cache",
		Run:   sendCmd,
	}
	statusHostsCmd = &cobra.Command{
		Use:   "hosts",
		Short: "Show hosts (may be a lot of output)",
		Run:   sendCmd,
	}
	statusRegexpsCmd = &cobra.Command{
		Use:   "regexps",
		Short: "Show regexps",
		Run:   sendCmd,
	}
	statusOverrideCmd = &cobra.Command{
		Use:   "override",
		Short: "Show override table",
		Run:   sendCmd,
	}
)

func init() {
	RootCmd.AddCommand(statusCmd)
	statusCmd.AddCommand(statusSummaryCmd)
	statusCmd.AddCommand(statusConfigCmd)
	statusCmd.AddCommand(statusCacheCmd)
	statusCmd.AddCommand(statusHostsCmd)
	statusCmd.AddCommand(statusRegexpsCmd)
	statusCmd.AddCommand(statusOverrideCmd)
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
