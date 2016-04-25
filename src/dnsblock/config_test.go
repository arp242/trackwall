package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

func mktmp(data string) (filename string) {
	fp, err := ioutil.TempFile(os.TempDir(), "dnsblock_test_")
	if err != nil {
		fmt.Printf("cannot create temporary file: %v\n", err)
		os.Exit(1)
	}
	defer fp.Close()

	_, err = fp.WriteString(data)
	if err != nil {
		fmt.Printf("cannot write to temporary file: %v\n", err)
		os.Exit(1)
	}

	return fp.Name()
}

func TestParse(t *testing.T) {
	var tmp string
	tmp = mktmp("dns-listen")

	_=tmp
	// fatal() will just exit, we should probably refactor that
	//(&config_t{}).parse(tmp)
}
