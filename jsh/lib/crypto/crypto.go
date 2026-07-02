package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	exports.Set("generateX509Certificate", generateX509Certificate)
	exports.Set("writeHostFile", writeHostFile)
}

type CertificateRequest struct {
	Days int      `json:"days"` // number of days the certificate is valid for
	CN   string   `json:"cn"`   // common name for the certificate
	O    []string `json:"o"`    // organization names for the certificate
	OU   []string `json:"ou"`   // organizational unit names for the certificate
	L    []string `json:"l"`    // localities for the certificate
	ST   []string `json:"st"`   // states or provinces for the certificate
	C    []string `json:"c"`    // country names for the certificate
	DNS  []string `json:"dns"`  // DNS names for the certificate
	URI  []string `json:"uri"`  // URIs for the certificate
	SAN  []string `json:"san"`  // SANs for the certificate
}

// generateX509Certificate generates an X.509 certificate based on the provided certificate request,
// public key, and signer private key.
// It returns the PEM-encoded certificate as a string, or an error if the certificate generation fails.
//
// - r is the certificate request containing CN, O, OU, L, ST, C, DNS, URI, and SAN fields for the certificate.
// - publicKey is the public key to be included in the certificate.
// - signerPrivateKey is the private key used to sign the certificate. For self-signed certificates, this is the same as the public key.
func generateX509Certificate(request map[string]any, publicKey string, signerPrivateKey string) (string, error) {
	r, err := normalizeCertificateRequest(request)
	if err != nil {
		return "", err
	}
	pub, err := parsePublicKeyPEM(publicKey)
	if err != nil {
		return "", err
	}
	signer, err := parsePrivateKeyPEM(signerPrivateKey)
	if err != nil {
		return "", err
	}
	if r.Days <= 0 {
		return "", fmt.Errorf("invalid certificate validity days %d", r.Days)
	}
	template := &x509.Certificate{
		SerialNumber:          mustSerialNumber(),
		Subject:               buildCertificateSubject(r),
		NotBefore:             time.Now().UTC().Add(-time.Minute),
		NotAfter:              time.Now().UTC().Add(time.Duration(r.Days) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	applyCertificateNames(template, r)
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, signer)
	if err != nil {
		return "", fmt.Errorf("create certificate: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})), nil
}

func normalizeCertificateRequest(request map[string]any) (CertificateRequest, error) {
	r := CertificateRequest{}
	if request == nil {
		return r, nil
	}
	r.CN = stringField(request, "cn", "CN")
	r.Days = intField(request, "days", "Days")
	r.O = stringSliceField(request, "o", "O")
	r.OU = stringSliceField(request, "ou", "OU")
	r.L = stringSliceField(request, "l", "L")
	r.ST = stringSliceField(request, "st", "ST")
	r.C = stringSliceField(request, "c", "C")
	r.DNS = stringSliceField(request, "dns", "DNS")
	r.URI = stringSliceField(request, "uri", "URI")
	r.SAN = stringSliceField(request, "san", "SAN")
	return r, nil
}

func stringField(request map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := request[key]; ok {
			if text, ok := value.(string); ok {
				return text
			}
			return fmt.Sprint(value)
		}
	}
	return ""
}

func intField(request map[string]any, keys ...string) int {
	for _, key := range keys {
		value, ok := request[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case int:
			return typed
		case int32:
			return int(typed)
		case int64:
			return int(typed)
		case float32:
			return int(typed)
		case float64:
			return int(typed)
		case uint:
			return int(typed)
		case uint32:
			return int(typed)
		case uint64:
			return int(typed)
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
				return n
			}
		}
	}
	return 0
}

func stringSliceField(request map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := request[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return typed
		case []interface{}:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				items = append(items, fmt.Sprint(item))
			}
			return items
		case string:
			if typed == "" {
				return nil
			}
			return []string{typed}
		default:
			return []string{fmt.Sprint(typed)}
		}
	}
	return nil
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

func parsePublicKeyPEM(value string) (any, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM public key")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		return key, nil
	}
	privateKey, err := parsePrivateKeyBlock(block)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key PEM block type %q", block.Type)
	}
	switch k := privateKey.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey, nil
	case *ecdsa.PrivateKey:
		return &k.PublicKey, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", privateKey)
	}
}

func parsePrivateKeyPEM(value string) (any, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}
	return parsePrivateKeyBlock(block)
}

func parsePrivateKeyBlock(block *pem.Block) (any, error) {
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("failed to parse private key PEM block type %q", block.Type)
}

func buildCertificateSubject(r CertificateRequest) pkix.Name {
	return pkix.Name{
		CommonName:         r.CN,
		Organization:       append([]string(nil), r.O...),
		OrganizationalUnit: append([]string(nil), r.OU...),
		Locality:           append([]string(nil), r.L...),
		Province:           append([]string(nil), r.ST...),
		Country:            append([]string(nil), r.C...),
	}
}

func applyCertificateNames(template *x509.Certificate, r CertificateRequest) {
	template.DNSNames = append(template.DNSNames, r.DNS...)
	for _, uriText := range r.URI {
		if uriText == "" {
			continue
		}
		if parsed, err := url.Parse(uriText); err == nil && parsed.Scheme != "" {
			template.URIs = append(template.URIs, parsed)
		}
	}
	for _, san := range r.SAN {
		addGeneralName(template, san)
	}
}

func addGeneralName(template *x509.Certificate, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	key, value, hasPrefix := strings.Cut(name, ":")
	if hasPrefix {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "dns":
			template.DNSNames = append(template.DNSNames, strings.TrimSpace(value))
			return
		case "ip":
			if ip := net.ParseIP(strings.TrimSpace(value)); ip != nil {
				template.IPAddresses = append(template.IPAddresses, ip)
			}
			return
		case "uri":
			if parsed, err := url.Parse(strings.TrimSpace(value)); err == nil && parsed.Scheme != "" {
				template.URIs = append(template.URIs, parsed)
			}
			return
		case "email":
			if strings.TrimSpace(value) != "" {
				template.EmailAddresses = append(template.EmailAddresses, strings.TrimSpace(value))
			}
			return
		}
	}
	if ip := net.ParseIP(name); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
		return
	}
	if parsed, err := url.Parse(name); err == nil && parsed.Scheme != "" {
		template.URIs = append(template.URIs, parsed)
		return
	}
	if strings.Contains(name, "@") {
		template.EmailAddresses = append(template.EmailAddresses, name)
		return
	}
	template.DNSNames = append(template.DNSNames, name)
}

func mustSerialNumber() *big.Int {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return big.NewInt(time.Now().UnixNano())
	}
	return serialNumber
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
