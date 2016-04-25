// Copyright © 2016 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

// The HTTP stuff
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"html"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
)

const _blocked = `<html><head><title> dnsblock %[1]s</title></head><body>
<p>dnsblock blocked access to <code>%[1]s</code>. Unblock this domain for:</p>
<ul><li><a href="/$@_allow/10s/%[2]s">ten seconds</a></li>
<li><a href="/$@_allow/1h/%[2]s">an hour</a></li>
<li><a href="/$@_allow/1d/%[2]s">a day</a></li>
<li><a href="/$@_allow/10y/%[2]s">until restart</a></li></ul></body></html>`

const _list = `<html><head><title>dnsblock</title></head><body><ul>
<li><a href="/$@_list/config">config</a></li>
<li><a href="/$@_list/hosts">hosts</a></li>
<li><a href="/$@_list/regexps">regexps</a></li>
<li><a href="/$@_list/override">override</a></li>
<li><a href="/$@_list/cache">cache</a></li>
</ul></body></html>`

func listenHttp() {
	go func() {
		err := http.ListenAndServe(_config.http_listen.String(), &handleHttp{})
		fatal(err)
	}()

	go func() {
		server := &http.Server{
			Addr:      _config.https_listen.String(),
			Handler:   &handleHttp{},
			TLSConfig: &tls.Config{GetCertificate: getCert},
		}

		err := server.ListenAndServeTLS("", "")
		fatal(err)
	}()
}

type handleHttp struct{}

func (f *handleHttp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for {
		if !_config.locked {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// See comment in handleDns
	drop_privs()

	host := html.EscapeString(r.Host)
	url := html.EscapeString(strings.TrimLeft(r.URL.Path, "/"))

	// Special $@_ control URL
	if strings.HasPrefix(url, "$@_") {
		// $@_allow/duration/redirect
		if strings.HasPrefix(url, "$@_allow") {
			params := strings.Split(url, "/")
			fmt.Println(params)
			secs, err := durationToSeconds(params[1])
			if err != nil {
				warn(err)
				return
			}

			_override_hosts[host] = time.Now().Add(time.Duration(secs) * time.Second).Unix()
			// TODO: Also add query parameters and such!
			w.Header().Set("Location", "/"+strings.Join(params[2:], "/"))
			w.WriteHeader(http.StatusSeeOther) // TODO: Is this the best 30x header?
			// $@_list/{config,hosts,override}
		} else if strings.HasPrefix(url, "$@_list") {
			params := strings.Split(url, "/")
			if len(params) < 2 || params[1] == "" {
				fmt.Fprintf(w, _list)
				return
			}

			param := params[1]
			switch param {
			case "config":
				spew.Fdump(w, _config)
			case "hosts":
				fmt.Fprintf(w, fmt.Sprintf("# Blocking %v hosts\n", len(_hosts)))
				for k, v := range _hosts {
					if v != "" {
						fmt.Fprintf(w, fmt.Sprintf("%v  # %v\n", k, v))
					} else {
						fmt.Fprintf(w, fmt.Sprintf("%v\n", k))
					}
				}
			case "regexps":
				for _, v := range _regexps {
					fmt.Fprintf(w, fmt.Sprintf("%v\n", v))
				}
			case "override":
				spew.Fdump(w, _override_hosts)
			case "cache":
				spew.Fdump(w, _cache)
			}
		} else {
			fmt.Fprintf(w, "unknown command: %v", url)
		}
	} else {
		// Check for surrogate script
		sur, exists := _hosts[host]

		// This should never happen
		// TODO: this does happen if we block by regexp; we need to do special
		// foo here
		if !exists {
			//warn(fmt.Errorf("host %v not in _hosts?!", host))
		}

		// TODO: Do something sane with the Content-Type header
		if sur != "" {
			fmt.Fprintf(w, sur)
		} else {
			if strings.HasSuffix(url, ".js") {
				fmt.Fprintf(w, fmt.Sprintf("/*" + _blocked + "*/", host, url))
			} else {
				fmt.Fprintf(w, fmt.Sprintf(_blocked, host, url))
			}
		}
	}
}

// Generate a certificate for the domain
// TODO: This can be a lot more efficient.
func getCert(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := clientHello.ServerName
	if name == "" {
		return nil, fmt.Errorf("no ServerName")
	}
	os.MkdirAll("/cache/certs", 0700)

	// Make a key
	// openssl genrsa -out s7.addthis.com.key 2048
	keyfile := fmt.Sprintf("/cache/certs/%s.key", name)
	if _, err := os.Stat(keyfile); os.IsNotExist(err) {
		info("    Making a key for " + name)
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}

		fp, err := os.Create(keyfile)
		if err != nil {
			return nil, err
		}

		pem.Encode(fp, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
		fp.Close()
	}

	// Make a csr
	// openssl req -new -key s7.addthis.com.key -out s7.addthis.com.csr
	csrfile := fmt.Sprintf("/cache/certs/%s.csr", name)
	if _, err := os.Stat(csrfile); os.IsNotExist(err) {
		info("    Making a csr for " + name)
		template := x509.CertificateRequest{}

		if ip := net.ParseIP(name); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, name)
		}

		fp, err := os.Open(keyfile)
		data, err := ioutil.ReadAll(fp)
		keypem, _ := pem.Decode(data)
		fp.Close()
		key, err := x509.ParsePKCS1PrivateKey(keypem.Bytes)

		csr, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
		fp, err = os.Create(csrfile)
		if err != nil {
			return nil, err
		}

		pem.Encode(fp, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr})
		fp.Close()
	}

	// Make a cert
	// openssl x509 -req -in s7.addthis.com.csr -CA rootCA.pem -CAkey rootCA.key -CAcreateserial -out s7.addthis.com.crt -days 500 -sha256
	certfile := fmt.Sprintf("/cache/certs/%s.crt", name)
	if _, err := os.Stat(certfile); os.IsNotExist(err) {
		info("    Making a cert for " + name)

		// Load root CA
		fp, err := os.Open(_config.root_cert)
		data, err := ioutil.ReadAll(fp)
		rootpem, _ := pem.Decode(data)
		fp.Close()
		rootcerts, err := x509.ParseCertificates(rootpem.Bytes)
		if err != nil {
			warn(err)
			return nil, err
		}
		rootcert := *rootcerts[0]

		// Load root key
		fp, err = os.Open(_config.root_key)
		data, err = ioutil.ReadAll(fp)
		rootpem, _ = pem.Decode(data)
		fp.Close()
		rootkey, err := x509.ParsePKCS1PrivateKey(rootpem.Bytes)
		if err != nil {
			warn(err)
			return nil, err
		}

		// Make cert
		serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		template := x509.Certificate{
			SerialNumber: serialNumber,
			Subject: pkix.Name{
				Organization: []string{"dnsblock"},
			},
			NotBefore: time.Now().Add(-24 * time.Hour),
			NotAfter:  time.Now().Add(24 * time.Hour * 365 * 10),

			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
		}
		if ip := net.ParseIP(name); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, name)
		}

		cert, err := x509.CreateCertificate(rand.Reader, &template, &rootcert, &rootkey.PublicKey, rootkey)
		if err != nil {
			warn(err)
			return nil, err
		}
		fp, err = os.Create(certfile)
		if err != nil {
			return nil, err
		}

		pem.Encode(fp, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
		fp.Close()

		_ = cert
	}

	// We can now use the files to make a TLS certificate
	info(fmt.Sprintf("tls %s, %s", certfile, keyfile))
	//tlscert, err := tls.LoadX509KeyPair(certfile, keyfile)
	tlscert, err := tls.LoadX509KeyPair(certfile, _config.root_key)
	if err != nil {
		warn(err)
		return nil, err
	}

	return &tlscert, nil
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
