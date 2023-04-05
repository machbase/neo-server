package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
	"golang.org/x/crypto/ssh"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func NewCertPool(certs ...*x509.Certificate) *x509.CertPool {
	rootCAs := x509.NewCertPool()
	for _, c := range certs {
		rootCAs.AddCert(c)
	}
	return rootCAs
}

func ReadCertPem(path string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("server cert file fail: %s", err)
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: " + err.Error())
	}

	return cert, err
}

func ReadRsaKeyPem(path string) (*rsa.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("server cert file fail: %s", err)
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("failed to parse key PEM")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key: " + err.Error())
	}

	rsakey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("filed to parse key: wrong rsa format")
	}
	return rsakey, err
}

func VerifyCert(dnsName string, cert *x509.Certificate, ca *x509.Certificate) error {
	roots := x509.NewCertPool()
	roots.AddCert(ca)

	opts := x509.VerifyOptions{
		DNSName: dnsName,
		Roots:   roots,
	}

	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("failed to verify certificate: " + err.Error())
	}

	return nil
}

type ClientCertReq struct {
	SubjectName  string
	NotBefore    time.Time
	NotAfter     time.Time
	CaCert       *x509.Certificate
	CaPrivateKey any
	Format       string
	CertOut      io.Writer
	KeyOut       io.Writer
	PfxOut       io.Writer
	PfxPassword  string

	SshAuthorizedKeyOut io.Writer
}

func GenClientCert(req *ClientCertReq) error {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %v", err)
	}

	reader := rand.Reader
	bitSize := 4096

	key, err := rsa.GenerateKey(reader, bitSize)
	if err != nil {
		return err
	}

	if req.SshAuthorizedKeyOut != nil {
		sshpub, err := ssh.NewPublicKey(key.Public())
		if err != nil {
			return errors.Wrap(err, "public key failed")
		}
		b := ssh.MarshalAuthorizedKey(sshpub)
		req.SshAuthorizedKeyOut.Write(b)
	}

	template := &x509.Certificate{
		IsCA:                  false,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject: pkix.Name{
			CommonName:         req.SubjectName,
			OrganizationalUnit: []string{"MyTeamt"},
			Organization:       []string{"MyCompany"},
			StreetAddress:      []string{"South Korea"},
			Locality:           []string{"Seoul"},
			Country:            []string{"KR"},
		},
		NotBefore:   req.NotBefore,
		NotAfter:    req.NotAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, req.CaCert, &key.PublicKey, req.CaPrivateKey)

	if err != nil {
		return err
	}

	if err := pem.Encode(req.CertOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	var keyBytes []byte

	switch req.Format {
	case "pkcs1":
		keyBytes = x509.MarshalPKCS1PrivateKey(key)
	default:
		keyBytes, _ = x509.MarshalPKCS8PrivateKey(key)
	}
	if err := pem.Encode(req.KeyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}

	if req.PfxOut != nil {
		////////////////////////////////////////////////////////////////////////////////
		// note: pkcs12 library has been forked into https://github.com/OutOfBedlam/go-pkcs12
		cert, err := x509.ParseCertificate(derBytes)
		if err != nil {
			return err
		}
		var pfxData []byte
		if pfxData, err = pkcs12.Encode(reader, key, cert, []*x509.Certificate{req.CaCert}, req.PfxPassword); err != nil {
			return err
		}
		if _, err = req.PfxOut.Write(pfxData); err != nil {
			return err
		}
	}

	return nil
}

func HashCertificate(cert *x509.Certificate) (string, error) {
	raw := cert.Raw
	b64str := base64.StdEncoding.EncodeToString(raw)
	b64str = strings.Trim(b64str, "\r\n ")

	sha := sha3.New256()
	sha.Write([]byte(b64str))
	return hex.EncodeToString(sha.Sum(nil)), nil
}
