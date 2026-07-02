package crypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"strings"
	"testing"
)

func TestIntField(t *testing.T) {
	request := map[string]any{
		"int":         int(7),
		"int32":       int32(8),
		"int64":       int64(9),
		"float32":     float32(10),
		"float64":     float64(11),
		"uint":        uint(12),
		"uint32":      uint32(13),
		"uint64":      uint64(14),
		"trimmed":     " 15 ",
		"bad":         "not-a-number",
		"fallback":    "16",
		"unsupported": true,
	}

	tests := []struct {
		name string
		keys []string
		want int
	}{
		{name: "int", keys: []string{"int"}, want: 7},
		{name: "int32", keys: []string{"int32"}, want: 8},
		{name: "int64", keys: []string{"int64"}, want: 9},
		{name: "float32", keys: []string{"float32"}, want: 10},
		{name: "float64", keys: []string{"float64"}, want: 11},
		{name: "uint", keys: []string{"uint"}, want: 12},
		{name: "uint32", keys: []string{"uint32"}, want: 13},
		{name: "uint64", keys: []string{"uint64"}, want: 14},
		{name: "trimmed string", keys: []string{"trimmed"}, want: 15},
		{name: "fallback key after bad string", keys: []string{"bad", "fallback"}, want: 16},
		{name: "unsupported type", keys: []string{"unsupported"}, want: 0},
		{name: "missing", keys: []string{"missing"}, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := intField(request, tc.keys...); got != tc.want {
				t.Fatalf("intField(%v) = %d, want %d", tc.keys, got, tc.want)
			}
		})
	}
}

func TestStringSliceField(t *testing.T) {
	request := map[string]any{
		"strings":     []string{"alpha", "beta"},
		"interfaces":  []interface{}{"alpha", 42, true},
		"single":      "value",
		"empty":       "",
		"scalar":      3.14,
		"nilFirst":    nil,
		"nilFallback": []string{"gamma"},
	}

	tests := []struct {
		name string
		keys []string
		want []string
	}{
		{name: "string slice", keys: []string{"strings"}, want: []string{"alpha", "beta"}},
		{name: "interface slice", keys: []string{"interfaces"}, want: []string{"alpha", "42", "true"}},
		{name: "single string", keys: []string{"single"}, want: []string{"value"}},
		{name: "empty string", keys: []string{"empty"}, want: nil},
		{name: "scalar fallback", keys: []string{"scalar"}, want: []string{"3.14"}},
		{name: "skip nil key", keys: []string{"nilFirst", "nilFallback"}, want: []string{"gamma"}},
		{name: "missing", keys: []string{"missing"}, want: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stringSliceField(request, tc.keys...)
			if len(got) != len(tc.want) {
				t.Fatalf("stringSliceField(%v) length = %d, want %d (%v)", tc.keys, len(got), len(tc.want), got)
			}
			for idx := range tc.want {
				if got[idx] != tc.want[idx] {
					t.Fatalf("stringSliceField(%v)[%d] = %q, want %q", tc.keys, idx, got[idx], tc.want[idx])
				}
			}
		})
	}
}

func TestGenerateAuthKeyPair(t *testing.T) {
	tests := []struct {
		name            string
		keyType         string
		wantPrivateType string
		assertPublicKey func(*testing.T, any)
	}{
		{
			name:            "default ecdsa",
			keyType:         "",
			wantPrivateType: "EC PRIVATE KEY",
			assertPublicKey: func(t *testing.T, key any) {
				t.Helper()
				if _, ok := key.(*ecdsa.PublicKey); !ok {
					t.Fatalf("expected ECDSA public key, got %T", key)
				}
			},
		},
		{
			name:            "trimmed rsa",
			keyType:         " RSA ",
			wantPrivateType: "RSA PRIVATE KEY",
			assertPublicKey: func(t *testing.T, key any) {
				t.Helper()
				if _, ok := key.(*rsa.PublicKey); !ok {
					t.Fatalf("expected RSA public key, got %T", key)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pair, err := generateAuthKeyPair(tc.keyType)
			if err != nil {
				t.Fatalf("generateAuthKeyPair(%q) error = %v", tc.keyType, err)
			}

			privateBlock, _ := pem.Decode([]byte(pair["privateKey"]))
			if privateBlock == nil {
				t.Fatalf("failed to decode private key PEM")
			}
			if privateBlock.Type != tc.wantPrivateType {
				t.Fatalf("private PEM type = %q, want %q", privateBlock.Type, tc.wantPrivateType)
			}

			publicBlock, _ := pem.Decode([]byte(pair["publicKey"]))
			if publicBlock == nil {
				t.Fatalf("failed to decode public key PEM")
			}
			if publicBlock.Type != "PUBLIC KEY" {
				t.Fatalf("public PEM type = %q, want PUBLIC KEY", publicBlock.Type)
			}

			publicKey, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
			if err != nil {
				t.Fatalf("parse public key: %v", err)
			}
			tc.assertPublicKey(t, publicKey)
		})
	}

	t.Run("invalid type", func(t *testing.T) {
		if _, err := generateAuthKeyPair("invalid"); err == nil {
			t.Fatal("expected error for invalid key type")
		}
	})
}

func TestAddGeneralName(t *testing.T) {
	template := &x509.Certificate{}

	addGeneralName(template, "")
	addGeneralName(template, " dns:example.com ")
	addGeneralName(template, "ip:127.0.0.1")
	addGeneralName(template, "uri:spiffe://machbase/neo")
	addGeneralName(template, "email:ops@example.com")
	addGeneralName(template, "10.0.0.8")
	addGeneralName(template, "https://example.com/service")
	addGeneralName(template, "admin@example.com")
	addGeneralName(template, "localhost")
	addGeneralName(template, "ip:not-an-ip")

	if got := template.DNSNames; len(got) != 2 || got[0] != "example.com" || got[1] != "localhost" {
		t.Fatalf("unexpected DNS names: %#v", got)
	}
	if got := template.IPAddresses; len(got) != 2 || !got[0].Equal(net.ParseIP("127.0.0.1")) || !got[1].Equal(net.ParseIP("10.0.0.8")) {
		t.Fatalf("unexpected IP addresses: %#v", got)
	}
	if got := template.URIs; len(got) != 2 || got[0].String() != "spiffe://machbase/neo" || got[1].String() != "https://example.com/service" {
		t.Fatalf("unexpected URIs: %#v", got)
	}
	if got := template.EmailAddresses; len(got) != 2 || got[0] != "ops@example.com" || got[1] != "admin@example.com" {
		t.Fatalf("unexpected email addresses: %#v", got)
	}
}

func TestGenerateX509CertificateErrors(t *testing.T) {
	pair, err := generateAuthKeyPair("ecdsa")
	if err != nil {
		t.Fatalf("generateAuthKeyPair(ecdsa) error = %v", err)
	}

	t.Run("invalid days", func(t *testing.T) {
		_, err := generateX509Certificate(map[string]any{"days": 0}, pair["publicKey"], pair["privateKey"])
		if err == nil || !strings.Contains(err.Error(), "invalid certificate validity days") {
			t.Fatalf("expected invalid days error, got %v", err)
		}
	})

	t.Run("invalid public key pem", func(t *testing.T) {
		_, err := generateX509Certificate(map[string]any{"days": 1}, "not a pem", pair["privateKey"])
		if err == nil || !strings.Contains(err.Error(), "failed to decode PEM public key") {
			t.Fatalf("expected public key PEM error, got %v", err)
		}
	})

	t.Run("invalid private key pem", func(t *testing.T) {
		_, err := generateX509Certificate(map[string]any{"days": 1}, pair["publicKey"], "not a pem")
		if err == nil || !strings.Contains(err.Error(), "failed to decode PEM private key") {
			t.Fatalf("expected private key PEM error, got %v", err)
		}
	})

	t.Run("signer does not match public key algorithm", func(t *testing.T) {
		x25519Key, err := ecdh.X25519().GenerateKey(nil)
		if err != nil {
			t.Fatalf("generate X25519 key: %v", err)
		}
		x25519DER, err := x509.MarshalPKCS8PrivateKey(x25519Key)
		if err != nil {
			t.Fatalf("marshal X25519 key: %v", err)
		}
		x25519PEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x25519DER}))

		_, err = generateX509Certificate(map[string]any{"days": 1}, pair["publicKey"], x25519PEM)
		if err == nil || !strings.Contains(err.Error(), "create certificate:") {
			t.Fatalf("expected create certificate error, got %v", err)
		}
	})
}
