package crypto_test

import (
	"os"
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
