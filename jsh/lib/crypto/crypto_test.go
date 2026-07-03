package crypto_test

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestGenerateAuthKeyPair(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "ecdsa",
			Script: `
				const crypto = require('crypto');
				const pair = crypto.generateAuthKeyPair('ecdsa');
				console.println(pair.privateKey.includes('BEGIN EC PRIVATE KEY'));
				console.println(pair.publicKey.includes('BEGIN PUBLIC KEY'));
			`,
			Output: []string{"true", "true"},
		},
		{
			Name: "rsa",
			Script: `
				const crypto = require('crypto');
				const pair = crypto.generateAuthKeyPair('rsa');
				console.println(pair.privateKey.includes('BEGIN RSA PRIVATE KEY'));
				console.println(pair.publicKey.includes('BEGIN PUBLIC KEY'));
			`,
			Output: []string{"true", "true"},
		},
		{
			Name: "invalid-type",
			Script: `
				const crypto = require('crypto');
				try {
					crypto.generateAuthKeyPair('invalid');
				} catch (err) {
					console.println(String(err).includes('unsupported key type'));
				}
			`,
			Output: []string{"true"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestWriteHostFile(t *testing.T) {
	hostFile := t.TempDir() + "/hello.txt"
	tc := test_engine.TestCase{
		Name: "write-host",
		Script: `
			const crypto = require('crypto');
			const process = require('process');
			const file = process.env.get('HOST_FILE');
			crypto.writeHostFile(file, 'hello', 0o600);
			console.println('ok');
		`,
		Vars:   map[string]any{"HOST_FILE": hostFile},
		Output: []string{"ok"},
	}
	test_engine.RunTest(t, tc)

	b, err := os.ReadFile(hostFile)
	if err != nil {
		t.Fatalf("read host file: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("unexpected host file content: %q", string(b))
	}
}

func TestGenerateX509Certificate(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "ecdsa",
			Script: `
				const crypto = require('crypto');
				const process = require('process');
				const pair = crypto.generateAuthKeyPair(process.env.get('KEY_TYPE'));
				const cert = crypto.generateX509Certificate({
					days: 30,
					cn: 'example.local',
					o: ['Machbase'],
					ou: ['JSH'],
					l: ['Seoul'],
					st: ['Seoul'],
					c: ['KR'],
					dns: ['example.local', 'localhost'],
					uri: ['spiffe://machbase/neo'],
					san: ['ip:127.0.0.1', 'email:ops@example.com'],
				}, pair.publicKey, pair.privateKey);
				console.println(JSON.stringify(cert));
			`,
			Vars: map[string]any{"KEY_TYPE": "ecdsa"},
			ExpectFunc: func(t *testing.T, result string) {
				t.Helper()
				assertCertificate(t, result)
			},
		},
		{
			Name: "rsa",
			Script: `
				const crypto = require('crypto');
				const process = require('process');
				const pair = crypto.generateAuthKeyPair(process.env.get('KEY_TYPE'));
				const cert = crypto.generateX509Certificate({
					days: 30,
					cn: 'example.local',
					o: ['Machbase'],
					ou: ['JSH'],
					l: ['Seoul'],
					st: ['Seoul'],
					c: ['KR'],
					dns: ['example.local', 'localhost'],
					uri: ['spiffe://machbase/neo'],
					san: ['ip:127.0.0.1', 'email:ops@example.com'],
				}, pair.publicKey, pair.privateKey);
				console.println(JSON.stringify(cert));
			`,
			Vars: map[string]any{"KEY_TYPE": "rsa"},
			ExpectFunc: func(t *testing.T, result string) {
				t.Helper()
				assertCertificate(t, result)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func assertCertificate(t *testing.T, result string) {
	t.Helper()
	var certPEM string
	if err := json.Unmarshal([]byte(strings.TrimSpace(result)), &certPEM); err != nil {
		t.Fatalf("unmarshal certificate PEM: %v", err)
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatalf("expected CERTIFICATE PEM block, got %q", certPEM)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	if cert.Subject.CommonName != "example.local" {
		t.Fatalf("unexpected common name: %q", cert.Subject.CommonName)
	}
	if got := cert.Subject.Organization; len(got) != 1 || got[0] != "Machbase" {
		t.Fatalf("unexpected organization: %#v", got)
	}
	if got := cert.Subject.OrganizationalUnit; len(got) != 1 || got[0] != "JSH" {
		t.Fatalf("unexpected organizational unit: %#v", got)
	}
	if got := cert.Subject.Locality; len(got) != 1 || got[0] != "Seoul" {
		t.Fatalf("unexpected locality: %#v", got)
	}
	if got := cert.Subject.Province; len(got) != 1 || got[0] != "Seoul" {
		t.Fatalf("unexpected province: %#v", got)
	}
	if got := cert.Subject.Country; len(got) != 1 || got[0] != "KR" {
		t.Fatalf("unexpected country: %#v", got)
	}
	if len(cert.DNSNames) != 2 || cert.DNSNames[0] != "example.local" || cert.DNSNames[1] != "localhost" {
		t.Fatalf("unexpected dns names: %#v", cert.DNSNames)
	}
	if len(cert.URIs) != 1 || cert.URIs[0].String() != "spiffe://machbase/neo" {
		t.Fatalf("unexpected uris: %#v", cert.URIs)
	}
	if len(cert.IPAddresses) != 1 || cert.IPAddresses[0].String() != "127.0.0.1" {
		t.Fatalf("unexpected ip addresses: %#v", cert.IPAddresses)
	}
	if len(cert.EmailAddresses) != 1 || cert.EmailAddresses[0] != "ops@example.com" {
		t.Fatalf("unexpected email addresses: %#v", cert.EmailAddresses)
	}
	if !cert.NotAfter.After(cert.NotBefore) {
		t.Fatalf("expected certificate validity window, got notBefore=%v notAfter=%v", cert.NotBefore, cert.NotAfter)
	}
}
