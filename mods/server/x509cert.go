package server

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/mods/model"
	"golang.org/x/crypto/sha3"
)

func GenerateServerCertificate(priKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) ([]byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{IsCA: true, BasicConstraintsValid: true, SerialNumber: serialNumber, Subject: pkix.Name{CommonName: "neo.machbase.com", OrganizationalUnit: []string{"R&D Center"}, Organization: []string{"machbase.com"}, StreetAddress: []string{"3003 N First St #206"}, PostalCode: []string{"95134"}, Locality: []string{"San Jose"}, Country: []string{"CA"}}, NotBefore: time.Now(), NotAfter: time.Now().AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, priKey)
	if err != nil {
		return nil, err
	}
	out := &bytes.Buffer{}
	if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func GenerateClientCertificate(subject pkix.Name, dnsNames []string, uris []*url.URL, notBefore time.Time, notAfter time.Time, ca *x509.Certificate, caKey crypto.PrivateKey, pubKey crypto.PublicKey) ([]byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{IsCA: false, BasicConstraintsValid: true, SerialNumber: serialNumber, Subject: subject, DNSNames: dnsNames, URIs: uris, NotBefore: notBefore, NotAfter: notAfter, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca, pubKey, caKey)
	if err != nil {
		return nil, err
	}
	out := &bytes.Buffer{}
	if err := pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func HashCertificate(cert *x509.Certificate) (string, error) {
	b64str := strings.Trim(base64.StdEncoding.EncodeToString(cert.Raw), "\r\n ")
	sha := sha3.New256()
	sha.Write([]byte(b64str))
	return hex.EncodeToString(sha.Sum(nil)), nil
}

func parseCertificatePEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}

type KeyInfo struct {
	Idx       int    `json:"idx"`
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	NotBefore int64  `json:"notBefore"`
	NotAfter  int64  `json:"notAfter"`
}
type GenKeyRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	NotBefore int64  `json:"notBefore"`
	NotAfter  int64  `json:"notAfter"`
	NotStore  bool   `json:"notStore"`
}
type GenKeyResponse struct {
	Success     bool   `json:"success"`
	Reason      string `json:"reason"`
	Elapse      string `json:"elapse"`
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Certificate string `json:"certificate"`
}
type DelKeyRequest struct {
	Id int64 `json:"id"`
}
type DelKeyResponse struct {
	Success bool   `json:"success"`
	Reason  string `json:"reason"`
	Elapse  string `json:"elapse"`
}
type ServerKeyRequest struct {
	Format string `json:"format,omitempty"`
}
type ServerKeyResponse struct {
	Success     bool   `json:"success"`
	Reason      string `json:"reason"`
	Elapse      string `json:"elapse"`
	Format      string `json:"format,omitempty"`
	Certificate string `json:"certificate"`
}

func (s *Server) GenKey(ctx context.Context, req *GenKeyRequest) (*GenKeyResponse, error) {
	tick := time.Now()
	rsp := &GenKeyResponse{Reason: "not specified"}
	defer func() { rsp.Elapse = time.Since(tick).String() }()
	req.Name = strings.ToLower(req.Name)
	pass, _ := regexp.MatchString("[a-z][a-z0-9_.@-]+", req.Name)
	if !pass {
		rsp.Reason = "name contains invalid character"
		return rsp, nil
	}
	if len(req.Name) > 40 {
		rsp.Reason = "name is too long, should be shorter than 40 characters"
		return rsp, nil
	}
	if s.models == nil {
		return nil, errors.New("model provider is not available")
	}
	scope, err := modelUserScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ca, err := s.ServerCertificate()
	if err != nil {
		return nil, err
	}
	caKey, err := s.ServerPrivateKey()
	if err != nil {
		return nil, err
	}
	clientURN, err := url.Parse(fmt.Sprintf("urn:machbase:neo:client:%s", req.Name))
	if err != nil {
		rsp.Reason = fmt.Sprintf("invalid client urn, %s", err.Error())
		return rsp, nil
	}
	cert, key, err := generateClientKey(&GenCertReq{Name: pkix.Name{CommonName: req.Name}, DNSNames: []string{req.Name}, URIs: []*url.URL{clientURN}, NotBefore: time.Unix(req.NotBefore, 0), NotAfter: time.Unix(req.NotAfter, 0), Issuer: ca, IssuerKey: caKey, Type: strings.ToLower(req.Type), Format: "pkcs8"})
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	if !req.NotStore {
		parsed, err := parseCertificatePEM(cert)
		if err != nil {
			return nil, err
		}
		hash, err := HashCertificate(parsed)
		if err != nil {
			return nil, err
		}
		definition := &model.X509CertDefinition{Name: req.Name, CertPEM: string(cert), CertHash: hash, KeyType: strings.ToLower(req.Type), NotBefore: parsed.NotBefore, NotAfter: parsed.NotAfter}
		if err := s.models.SaveX509Cert(ctx, scope, definition); err != nil {
			rsp.Reason = err.Error()
			return rsp, nil
		}
		rsp.Id = definition.Id
		if s.x509CertVerifier != nil {
			s.x509CertVerifier.Invalidate(hash)
		}
	}
	rsp.Name, rsp.Certificate, rsp.Key, rsp.Success, rsp.Reason = req.Name, string(cert), string(key), true, "success"
	return rsp, nil
}

func (s *Server) DelKey(ctx context.Context, req *DelKeyRequest) (*DelKeyResponse, error) {
	tick := time.Now()
	rsp := &DelKeyResponse{}
	defer func() { rsp.Elapse = time.Since(tick).String() }()
	if s.models == nil {
		return nil, errors.New("model provider is not available")
	}
	scope, err := modelUserScopeFromContext(ctx)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	certHash, err := s.models.RemoveX509Cert(ctx, scope, req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	if s.x509CertVerifier != nil {
		s.x509CertVerifier.Invalidate(certHash)
	}
	return rsp, nil
}

func (s *Server) ServerKey(ctx context.Context, req *ServerKeyRequest) (*ServerKeyResponse, error) {
	tick := time.Now()
	rsp := &ServerKeyResponse{Reason: "unspecified"}
	defer func() { rsp.Elapse = time.Since(tick).String() }()
	if req == nil {
		req = &ServerKeyRequest{Format: "pem"}
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "pem"
	}
	rsp.Format = strings.ToUpper(format)
	b, err := os.ReadFile(s.ServerCertificatePath())
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	if format == "pem" {
		rsp.Success, rsp.Reason, rsp.Certificate = true, "success", string(b)
		return rsp, nil
	}
	block, _ := pem.Decode(b)
	if block == nil {
		rsp.Reason = "invalid PEM certificate"
		return rsp, nil
	}
	switch format {
	case "der", "cer":
		rsp.Success, rsp.Reason, rsp.Certificate = true, "success", base64.StdEncoding.EncodeToString(block.Bytes)
	default:
		rsp.Reason = "unsupported format, use PEM, DER, or CER"
	}
	return rsp, nil
}

type GenCertReq struct {
	pkix.Name
	DNSNames  []string
	URIs      []*url.URL
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    *x509.Certificate
	IssuerKey any
	Type      string
	Format    string
}

func generateClientKey(req *GenCertReq) ([]byte, []byte, error) {
	var clientKey, clientPub any
	var clientKeyPEM []byte
	switch {
	case strings.HasPrefix(strings.ToLower(req.Type), "rsa"):
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, nil, err
		}
		clientKey, clientPub = key, &key.PublicKey
		var keyBytes []byte
		if req.Format == "pkcs1" {
			keyBytes = x509.MarshalPKCS1PrivateKey(key)
		} else {
			keyBytes, _ = x509.MarshalPKCS8PrivateKey(clientKey)
		}
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	case strings.HasPrefix(strings.ToLower(req.Type), "ec"):
		ec := NewEllipticCurveP256()
		pri, pub, err := ec.GenerateKeys()
		if err != nil {
			return nil, nil, err
		}
		clientKey, clientPub = pri, pub
		marshal, err := x509.MarshalECPrivateKey(pri)
		if err != nil {
			return nil, nil, err
		}
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: marshal})
	default:
		return nil, nil, errors.New("unsupported key type")
	}
	certBytes, err := GenerateClientCertificate(req.Name, req.DNSNames, req.URIs, req.NotBefore, req.NotAfter, req.Issuer, req.IssuerKey, clientPub)
	if err != nil {
		return nil, nil, fmt.Errorf("client certificate: %s", err.Error())
	}
	return certBytes, clientKeyPEM, nil
}
