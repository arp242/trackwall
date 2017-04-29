// Copyright © 2016-2017 Martin Tournoij <martin@arp242.net>
// See the bottom of this file for the full copyright notice.

package srvhttp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"time"

	"arp242.net/trackwall/cfg"
	"arp242.net/trackwall/msg"
)

// Generate a certificate for the domain
// TODO: This can be a lot more efficient.
// TODO: certs written out are world-readable
func getCert(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	name := clientHello.ServerName
	if name == "" {
		return nil, fmt.Errorf("no ServerName")
	}
	if err := os.MkdirAll("/cache/certs", 0700); err != nil {
		return nil, err
	}

	keyfile := fmt.Sprintf("/cache/certs/%s.key", name)
	if _, err := os.Stat(keyfile); os.IsNotExist(err) {
		err := makeKey(name, keyfile)
		if err != nil {
			return nil, fmt.Errorf("cannot make key: %v", err)
		}
	}

	csrfile := fmt.Sprintf("/cache/certs/%s.csr", name)
	if _, err := os.Stat(csrfile); os.IsNotExist(err) {
		err := makeCSR(name, keyfile, csrfile)
		if err != nil {
			return nil, fmt.Errorf("cannot make CSR: %v", err)
		}
	}

	certfile := fmt.Sprintf("/cache/certs/%s.crt", name)
	if _, err := os.Stat(certfile); os.IsNotExist(err) {
		err := makeCert(name, certfile)
		if err != nil {
			return nil, fmt.Errorf("cannot make certificate: %v", err)
		}
	}

	// We can now use the files to make a TLS certificate
	msg.Debug(fmt.Sprintf("tls %s, %s", certfile, keyfile))
	//tlscert, err := tls.LoadX509KeyPair(certfile, keyfile)
	tlscert, err := tls.LoadX509KeyPair(certfile, cfg.Config.RootKey)
	if err != nil {
		msg.Warn(err)
		return nil, err
	}

	return &tlscert, nil
}

// Make a key
// openssl genrsa -out s7.addthis.com.key 2048
func makeKey(name, keyfile string) error {
	msg.Debug("    Making a key for " + name)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	fp, err := os.Create(keyfile)
	if err != nil {
		return err
	}

	err = pem.Encode(fp, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err != nil {
		return err
	}

	_ = fp.Close()
	return nil
}

// Make a csr
// openssl req -new -key s7.addthis.com.key -out s7.addthis.com.csr
func makeCSR(name, keyfile, csrfile string) error {
	msg.Debug("    Making a csr for " + name)
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
		return err
	}

	pem.Encode(fp, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr})
	fp.Close()
	return nil
}

// Make a cert
// openssl x509 -req -in s7.addthis.com.csr -CA rootCA.pem -CAkey rootCA.key -CAcreateserial -out s7.addthis.com.crt -days 500 -sha256
func makeCert(name, certfile string) error {
	msg.Debug("    Making a cert for " + name)

	// Load root CA
	fp, err := os.Open(cfg.Config.RootCert)
	data, err := ioutil.ReadAll(fp)
	rootpem, _ := pem.Decode(data)
	fp.Close()
	rootcerts, err := x509.ParseCertificates(rootpem.Bytes)
	if err != nil {
		msg.Warn(err)
		return err
	}
	rootcert := *rootcerts[0]

	// Load root key
	fp, err = os.Open(cfg.Config.RootKey)
	data, err = ioutil.ReadAll(fp)
	rootpem, _ = pem.Decode(data)
	fp.Close()
	rootkey, err := x509.ParsePKCS1PrivateKey(rootpem.Bytes)
	if err != nil {
		msg.Warn(err)
		return err
	}

	// Make cert
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "trackwall root",
			Organization: []string{"trackwall"},
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
		msg.Warn(err)
		return err
	}
	fp, err = os.Create(certfile)
	if err != nil {
		return err
	}

	pem.Encode(fp, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	_ = fp.Close()
	return nil
}

// MakeRootKey makes a new root key.
// openssl genrsa -out /var/trackwall/rootCA.key 2048
// NOTE: Assumes that it is run *BEFORE* chroot(). See chroot() in main.go
func MakeRootKey() {
	msg.Warn(fmt.Errorf("generating a new root key at %s", cfg.Config.RootKey))

	p := cfg.Config.ChrootDir(cfg.Config.RootKey)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	msg.Fatal(err)

	fp, err := os.Create(p)
	msg.Fatal(err)

	pem.Encode(fp, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	fp.Close()

	msg.Fatal(os.Chmod(p, os.FileMode(0600)))
}

// MakeRootCert makes a new root certificate.
// openssl req -x509 -new -nodes -key /var/trackwall/rootCA.key -sha256 -days 1024 -out /var/trackwall/rootCA.pem
// NOTE: Assumes that it is run *BEFORE* chroot(). See chroot() in main.go
func MakeRootCert() {
	msg.Warn(fmt.Errorf("generating a new root certificate at %s", cfg.Config.RootCert))

	key := cfg.Config.ChrootDir(cfg.Config.RootKey)
	p := cfg.Config.ChrootDir(cfg.Config.RootCert)

	fp, err := os.Open(key)
	data, err := ioutil.ReadAll(fp)
	rootpem, _ := pem.Decode(data)
	fp.Close()
	rootkey, err := x509.ParsePKCS1PrivateKey(rootpem.Bytes)
	msg.Fatal(err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "trackwall root",
			Organization: []string{"trackwall root"},
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour * 365 * 10),
		BasicConstraintsValid: true,
		IsCA: true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &rootkey.PublicKey, rootkey)
	msg.Fatal(err)
	fp, err = os.Create(p)
	msg.Fatal(err)

	pem.Encode(fp, &pem.Block{Type: "CERTIFICATE", Bytes: cert})
	fp.Close()

	msg.Fatal(os.Chmod(p, os.FileMode(0600)))
}

// Install in system
//
// http://kb.kerio.com/product/kerio-connect/server-configuration/ssl-certificates/adding-trusted-root-certificates-to-the-server-1605.html
//
// update-ca-trust force-enable
// ln -s /var/trackwall/rootCA.pem /etc/ca-certificates/trust-source/anchors/
// update-ca-trust extract
//
func installRootCert() {
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
