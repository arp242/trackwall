What does a config file look like?
----------------------------------

	# This is a comment

	port 8080 # This is also a comment

	# Look ma, no quotes!
	base-url http://example.com

	# We'll parse these in a []*regexp.Regexp
	match ^foo
	match ^bar

	# Two values
	order allow deny


What does the code look like?
-----------------------------

	package main

	import (
		"fmt"
		"os"

		"code.arp242.net/sconfig"
	)

	type Config struct {
		Port    int
		BaseURL string
	}

	var config Config
	func main() {
		err := config.Parse("config")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing config: %v", err)
		}

		fmt.Printf("%#v\n", config)
	}


But why not...
--------------

- JSON?  
  JSON is not at all suitable for configuration files. Also see: [JSON as
  configuration files: please don’t][json-no].
- YAML?  
  I don't like the whitespace significance in config files, and YAML can have
  some really weird behaviour. Also see: [TODO][yaml-no].
- XML?  

		<?xml version="1.0"?>
		<faq>
			<header level="2">
				<content>But why not...</content>
			</header>
			<items type="list">
				<item>
					<question>XML?</question>
					<answer type="string" adjective="Very" fullstop="true">funny</answer>
				</item>
			</items>
		</faq>

- INI or TOML?  
  They're both fine, I just don't like the syntax much. Typing all those pesky
  `=` and `"` characters is just so much work man!

Programs using it
-----------------
- [dnsblock][dnsblock]
- [urlview-ng][urlview-ng]
- [whatwiki][whatwiki]

Alternatives
------------

- https://github.com/kovetskiy/ko


[json-no]: http://arp242.net/weblog/JSON_as_configuration_files-_please_dont.html
[yaml-no]: http://TODO
[dnsblock]: http://code.arp242.net/dnsblock
[urlview-ng]: http://code.arp242.net/urlview-ng
[whatwiki]: http://code.arp242.net/whatwiki


