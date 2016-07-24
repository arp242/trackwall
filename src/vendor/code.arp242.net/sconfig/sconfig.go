// Package sconfig is a simple and functional configuration file parser.
package sconfig

import (
	"bufio"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"bitbucket.org/pkg/inflect"
)

// Debug makes this package print out debug statements to stderr
var Debug = false

// TypeHandlers can be used to handle types other than the basic builtin ones
var TypeHandlers = make(map[string]TypeHandler)

// TypeHandler takes the field to set and the value to set it to. It is expected
// to return the value to set it to.
type TypeHandler func(*reflect.Value, interface{}) interface{}

// Handlers can be used to run special code for a field. The map key is the name
// of the field in the struct. The function takes the unprocessed line split by
// whitespace and with the option name removed.
type Handlers map[string]func(line []string)

// readFile will read a file, strip comments, and collapse indents. This also
// deals with the special "source" command.
func readFile(file string) (lines [][]string, err error) {
	fp, err := os.Open(file)
	if err != nil {
		return lines, err
	}
	defer func() { _ = fp.Close() }()

	i := 0
	no := 0
	for scanner := bufio.NewScanner(fp); scanner.Scan(); {
		no++
		line := scanner.Text()

		isIndented := len(line) > 0 && unicode.IsSpace(rune(line[0]))
		line = strings.TrimSpace(line)

		if line == "" || line[0] == '#' {
			continue
		}

		// Ignore comments
		cmt := strings.Index(line, "#")
		if cmt > -1 {
			// Allow escaping # with \#
			if line[cmt-1] == '\\' {
				line = line[:cmt-1] + line[cmt:]
			} else {
				line = line[:cmt]
			}
		}

		// Collapse whitespace
		// TODO: Allow \  to prevent this
		line = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(line, " "))

		if isIndented {
			lines[i-1][1] += " " + line
		} else {
			// Source
			if strings.HasPrefix(line, "source ") {
				sourced, err := readFile(line[7:])
				if err != nil {
					return [][]string{}, err
				}
				lines = append(lines, sourced...)
			} else {
				lines = append(lines, []string{fmt.Sprintf("%d", no), line})
			}
			i++
		}
	}

	return lines, nil
}

// Parse a file.
func Parse(c interface{}, file string, handlers map[string]func(line []string)) error {
	lines, err := readFile(file)
	if err != nil {
		return err
	}

	values := reflect.ValueOf(c).Elem()
	types := reflect.TypeOf(c).Elem()
	for _, line := range lines {
		src := file + ":" + line[0]

		v := strings.Split(line[1], " ")

		// Get the variable name
		fieldName := inflect.Camelize(v[0])

		// TODO: Maybe find better inflect package?
		// This list is from golint
		acr := []string{"Api", "Ascii", "Cpu", "Css", "Dns", "Eof", "Guid", "Html",
			"Https", "Http", "Id", "Ip", "Json", "Lhs", "Qps", "Ram", "Rhs",
			"Rpc", "Sla", "Smtp", "Sql", "Ssh", "Tcp", "Tls", "Ttl", "Udp",
			"Ui", "Uid", "Uuid", "Uri", "Url", "Utf8", "Vm", "Xml", "Xsrf",
			"Xss"}
		for _, a := range acr {
			fieldName = strings.Replace(fieldName, a, strings.ToUpper(a), -1)
		}

		// TODO: Allow a tag to set the rule to match:
		// Foobar string `rule:"foo-bar"`

		// TODO: Also validate?
		// Foobar string `required:"true" format:"%d""`

		field := values.FieldByName(fieldName)
		if !field.CanAddr() {
			// Check plural version too
			fieldName = inflect.Pluralize(fieldName)
			field = values.FieldByName(fieldName)

			if !field.CanAddr() {
				return fmt.Errorf("%s: unknown option %s (field %s is missing)",
					src, v[0], fieldName)
			}
		}

		// Use handler
		// TODO: We should be able to use sconfig.One() or some such so we get
		// an error.
		// Or maybe use a tag for this?
		dbg(fieldName + strings.Repeat(" ", 15-len(fieldName)))
		handler, has := handlers[fieldName]
		if has {
			// TODO: Better dbg
			dbg("handler\t=> \n")
			handler(v[1:])
			continue
		}

		// Parse according to the tag
		t, _ := types.FieldByName(fieldName)
		tag := t.Tag.Get("sconfig")
		if tag == "" {
			tag = "all"
		}

		var value interface{}
		switch tag {
		case "one":
			if len(v) < 2 || len(v) > 2 {
				return fmt.Errorf("%s: the %s option takes exactly one value (%v given)",
					src, v[0], len(v)-1)
			}
			value = v[1]
		case "two":
			if len(v) < 2 {
				return fmt.Errorf("%s: the %s option takes at least one values (%v given)",
					src, v[0], len(v)-1)
			}
			//value = v[1:]
		case "three":
			if len(v) < 3 {
				return fmt.Errorf("%s: the %s option takes at least three values (%v given)",
					src, v[0], len(v)-1)
			}
			//value = v[2:]
		case "all":
			value = strings.Join(v[1:], " ")
		default:
			return fmt.Errorf("%s: unknown type %s for %s", src, tag, fieldName)
		}

		err := setValue(&field, value)
		dbg("\n")
		if err != nil {
			return err
		}
	}

	return nil
}

// setValue sets the struct field to the given value. The value will be
// type-coerced in the field's type.
func setValue(field *reflect.Value, value interface{}) error {
	switch field.Interface().(type) {
	// Primitives
	case int, int8, int16, int32, int64:
		i, err := strconv.ParseInt(value.(string), 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(i)
		dbg("%s\t=> %#v", field.Type(), i)
	case uint, uint8, uint16, uint32, uint64:
		i, err := strconv.ParseUint(value.(string), 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(i)
		dbg("%s\t=> %#v", field.Type(), i)
	case bool:
		s := strings.ToLower(value.(string))
		switch s {
		case "true", "yes", "on", "enable", "enabled":
			field.SetBool(true)
			dbg("%s\t=> %#v", field.Type(), true)
		case "false", "no", "off", "disable", "disabled":
			field.SetBool(false)
			dbg("%s\t=> %#v", field.Type(), false)
		default:
			return fmt.Errorf("unable to parse %s as a boolean", value)
		}
	case float32, float64:
		field.SetFloat(value.(float64))
		dbg("%s\t=> %#v", field.Type(), value.(float64))
	case string:
		field.SetString(value.(string))
		dbg("%s\t=> %#v", field.Type(), value.(string))

	default:
		str := field.Type().String()

		// Try to get it from TypeHandlers
		if fun, has := TypeHandlers[str]; has {
			v := fun(field, value)
			dbg("%s\t=> %#v", field.Type(), v)
			field.Set(reflect.ValueOf(v))

			return nil
		}

		// TODO: Make this go back to the code above
		if strings.HasPrefix(str, "[]") {
			switch field.Interface().(type) {
			case []int, []int8, []int16, []int32, []int64:
				i, err := strconv.ParseInt(value.(string), 10, 64)
				if err != nil {
					return err
				}
				field.Set(reflect.Append(*field, reflect.ValueOf(i)))
				dbg("%s\ta> %#v", field.Type(), i)
			case []uint, []uint8, []uint16, []uint32, []uint64:
				i, err := strconv.ParseUint(value.(string), 10, 64)
				if err != nil {
					return err
				}
				field.Set(reflect.Append(*field, reflect.ValueOf(i)))
				dbg("%s\ta> %#v", field.Type(), i)
			case []bool:
				s := strings.ToLower(value.(string))
				switch s {
				case "true", "yes", "on", "enable", "enabled":
					field.Set(reflect.Append(*field, reflect.ValueOf(true)))
					dbg("%s\ta> %#v", field.Type(), true)
				case "false", "no", "off", "disable", "disabled":
					field.Set(reflect.Append(*field, reflect.ValueOf(false)))
					dbg("%s\ta> %#v", field.Type(), false)
				default:
					return fmt.Errorf("unable to parse %s as a boolean", value)
				}
			case []float32, []float64:
				field.Set(reflect.Append(*field, reflect.ValueOf(value.(float64))))
				dbg("%s\ta> %#v", field.Type(), value.(float64))
			case []string:
				field.Set(reflect.Append(*field, reflect.ValueOf(value)))
				dbg("%s\ta> %#v", field.Type(), value.(string))
			default:
				// TODO: If the value is a struct, check if it has this field

				// Give up :-(
				return fmt.Errorf("don't know how to set fields of the type %s", str)

			}
		}
	}

	return nil
}

// dbg acts like fmt.Fprintf(os.Stderr, ...) if Debug is true.
func dbg(format string, a ...interface{}) {
	if !Debug {
		return
	}
	fmt.Fprintf(os.Stderr, format, a...)
}

// FindConfig tries to find a config file at the usual locations (in this
// order):
//
//   ~/.config/$file
//   ~/.$file
//   /etc/$file
//   /usr/local/etc/$file
//   /usr/pkg/etc/$file
//   ./$file
//
//   c.FindConfig("mypackage/config")
//
func FindConfig(file string) string {
	file = strings.TrimLeft(file, "/")

	locations := []string{}
	if xdg := os.Getenv("XDG_CONFIG"); xdg != "" {
		locations = append(locations, strings.TrimRight(xdg, "/")+"/"+file)
	}
	if home := os.Getenv("HOME"); home != "" {
		locations = append(locations, home+"/."+file)
	}

	locations = append(locations, []string{
		"/etc/" + file,
		"/usr/local/etc/" + file,
		"/usr/pkg/etc/" + file,
		"./" + file,
	}...)

	for _, l := range locations {
		if _, err := os.Stat(l); err == nil {
			return l
		}
	}

	return ""
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
