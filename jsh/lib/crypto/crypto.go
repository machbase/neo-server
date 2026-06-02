package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dop251/goja"
)

//go:embed crypto.js
var cryptoJS []byte

func Files() map[string][]byte {
	return map[string][]byte{
		"crypto.js": cryptoJS,
	}
}

func Module(_ context.Context, _ *goja.Runtime, module *goja.Object) {
	exports := module.Get("exports").(*goja.Object)
	exports.Set("generateAuthKeyPair", generateAuthKeyPair)
	exports.Set("writeHostFile", writeHostFile)
}

func generateAuthKeyPair(keyType string) (map[string]string, error) {
	switch strings.ToLower(strings.TrimSpace(keyType)) {
	case "", "ecdsa":
		privateKey, publicKey, err := generateECDSAP256()
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"privateKey": privateKey,
			"publicKey":  publicKey,
		}, nil
	case "rsa":
		privateKey, publicKey, err := generateRSA2048()
		if err != nil {
			return nil, err
		}
		return map[string]string{
			"privateKey": privateKey,
			"publicKey":  publicKey,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q, expected rsa or ecdsa", keyType)
	}
}

func generateECDSAP256() (string, string, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}
	privateDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return string(privatePEM), string(publicPEM), nil
}

func generateRSA2048() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return string(privatePEM), string(publicPEM), nil
}

func writeHostFile(path string, content string, perm int64) error {
	resolved := filepath.Clean(strings.TrimSpace(path))
	if resolved == "" || resolved == "." {
		return fmt.Errorf("invalid host file path")
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(perm)
	if mode == 0 {
		mode = 0o600
	}
	return os.WriteFile(resolved, []byte(content), mode)
}
