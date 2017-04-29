// Copyright © 2016-2017 Martin Tournoij
// See the bottom of this file for the full copyright.

// Package time TODO
package time // import "arp242.net/sconfig/handlers/time"

import (
	"time"

	"arp242.net/sconfig"
)

func init() {
	sconfig.RegisterType("duration", handleDuration)
	sconfig.RegisterType("location", handleLocation)
	sconfig.RegisterType("time", handleTime)
}

func handleDuration(v []string) (interface{}, error) {
	return time.Duration(0), nil
}

func handleLocation(v []string) (interface{}, error) {
	return time.Location{}, nil
}

func handleTime(v []string) (interface{}, error) {
	return time.Time{}, nil
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
