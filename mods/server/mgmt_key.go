package server

import (
	"bytes"
	"context"
	"crypto"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"

	"math/big"

	"github.com/machbase/neo-grpc/mgmt"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

func (s *svr) ListKey(context.Context, *mgmt.ListKeyRequest) (*mgmt.ListKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ListKeyResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.IterateAuthorizedCertificates(func(id string) bool {
		cert, err := s.AuthorizedCertificate(id)
		if err != nil {
			s.log.Warnf("fail to load certificate '%s', %s", id, err.Error())
			return true
		}
		if id != cert.Subject.CommonName {
			s.log.Warnf("certificate id '%s' has different common name '%s'", id, cert.Subject.CommonName)
			return true
		}

		item := mgmt.KeyInfo{
			Id:        cert.Subject.CommonName,
			NotBefore: cert.NotBefore.Unix(),
			NotAfter:  cert.NotAfter.Unix(),
		}

		rsp.Keys = append(rsp.Keys, &item)
		return true
	})
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

func (s *svr) GenKey(ctx context.Context, req *mgmt.GenKeyRequest) (*mgmt.GenKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.GenKeyResponse{Reason: "not specified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	req.Id = strings.ToLower(req.Id)
	pass, _ := regexp.MatchString("[a-z][a-z0-9_.@-]+", req.Id)
	if !pass {
		rsp.Reason = fmt.Sprintf("id contains invalid character")
		return rsp, nil
	}
	if len(req.Id) > 40 {
		rsp.Reason = fmt.Sprintf("id is too long, should be shorter than 40 characters")
		return rsp, nil
	}
	_, err := s.AuthorizedCertificate(req.Id)
	if err != nil && err != os.ErrNotExist {
		if err == os.ErrExist {
			rsp.Reason = fmt.Sprintf("'%s' already exists", req.Id)
		} else {
			rsp.Reason = err.Error()
		}
		return rsp, nil
	}

	ca, err := s.ServerCertificate()
	if err != nil {
		return nil, err
	}
	caKey, err := s.ServerPrivateKey()
	if err != nil {
		return nil, err
	}
	gen := GenCertReq{
		Name: pkix.Name{
			CommonName: req.Id,
		},
		NotBefore: time.Unix(req.NotBefore, 0),
		NotAfter:  time.Unix(req.NotAfter, 0),
		Issuer:    ca,
		IssuerKey: caKey,
		Type:      req.Type,
		Format:    "pkcs8",
	}
	cert, key, token, err := generateClientKey(&gen)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	s.SetAuthorizedCertificate(req.Id, cert)

	rsp.Id = req.Id
	rsp.Token = string(token)
	rsp.Certificate = string(cert)
	rsp.Key = string(key)
	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

func (s *svr) DelKey(ctx context.Context, req *mgmt.DelKeyRequest) (*mgmt.DelKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.DelKeyResponse{}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()

	err := s.RemoveAuthorizedCertificate(req.Id)
	if err != nil {
		rsp.Reason = err.Error()
		return rsp, nil
	}

	rsp.Success, rsp.Reason = true, "success"
	rsp.Elapse = time.Since(tick).String()
	return rsp, nil
}

type GenCertReq struct {
	pkix.Name
	NotBefore time.Time
	NotAfter  time.Time
	Issuer    *x509.Certificate
	IssuerKey any
	Type      string // rsa
	Format    string // pkcs1, pkcs8
}

/*
generateClientKey returns certificate, privatekey, token and error
*/
func generateClientKey(req *GenCertReq) ([]byte, []byte, string, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "failed to generate serial number")
	}

	reader := rand.Reader

	var clientKey any
	var clientPub any
	switch req.Type {
	case "rsa":
		bitSize := 4096
		key, err := rsa.GenerateKey(reader, bitSize)
		if err != nil {
			return nil, nil, "", err
		}
		if err != nil {
			return nil, nil, "", err
		}
		clientKey = key
		clientPub = &key.PublicKey
	case "ec", "ecdsa":
		ec := NewEllipticCurveP521()
		pri, pub, err := ec.GenerateKeys()
		if err != nil {
			return nil, nil, "", err
		}
		clientKey = pri
		clientPub = pub
	default:
		return nil, nil, "", errors.New("unsupported key type")
	}

	token, err := GenerateClientToken(req.CommonName, clientKey, "b")
	if err != nil {
		return nil, nil, "", err
	}

	template := &x509.Certificate{
		IsCA:                  false,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject:               req.Name,
		NotBefore:             req.NotBefore,
		NotAfter:              req.NotAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, req.Issuer, clientPub, req.IssuerKey)
	if err != nil {
		return nil, nil, "", err
	}
	certBuf := bytes.NewBuffer(nil)
	if err := pem.Encode(certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, "", err
	}

	var keyBytes []byte
	switch req.Format {
	case "pkcs1":
		if _, ok := clientKey.(*rsa.PrivateKey); ok {
			keyBytes = x509.MarshalPKCS1PrivateKey(clientKey.(*rsa.PrivateKey))
		} else {
			return nil, nil, "", fmt.Errorf("%s key type can not encoded into pkcs1 format", req.Type)
		}
	default:
		keyBytes, _ = x509.MarshalPKCS8PrivateKey(clientKey)
	}
	keyBuf := bytes.NewBuffer(nil)
	header := fmt.Sprintf("%s PRIVATE KEY", strings.ToUpper(req.Type))
	if err := pem.Encode(keyBuf, &pem.Block{Type: header, Bytes: keyBytes}); err != nil {
		return nil, nil, "", err
	}

	return certBuf.Bytes(), keyBuf.Bytes(), token, nil
}

func hashCertificate(cert *x509.Certificate) (string, error) {
	raw := cert.Raw
	b64str := base64.StdEncoding.EncodeToString(raw)
	b64str = strings.TrimSpace(b64str)

	sha := sha3.New256()
	sha.Write([]byte(b64str))
	return hex.EncodeToString(sha.Sum(nil)), nil
}

func GenerateClientToken(clientId string, priKey crypto.PrivateKey, method string) (token string, err error) {
	var secret []byte
	switch key := priKey.(type) {
	case *rsa.PrivateKey:
		secret = x509.MarshalPKCS1PrivateKey(key)
	case *ecdsa.PrivateKey:
		secret = elliptic.Marshal(key.Curve, key.X, key.Y)
	default:
		return "", fmt.Errorf("unsupported algorithm '%T'", key)
	}
	if method != "b" {
		return "", fmt.Errorf("unsupported method '%s'", method)
	}
	hash := hmac.New(sha256.New, secret)
	io.WriteString(hash, clientId)
	token = fmt.Sprintf("%s:%s:%s", clientId, method, hex.EncodeToString(hash.Sum(nil)))
	return
}

func VerifyClientToken(token string, priKey crypto.PrivateKey) (bool, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return false, errors.New("invalid token format")
	}
	clientId := parts[0]
	method := parts[1]
	verify, err := GenerateClientToken(clientId, priKey, method)
	if err != nil {
		return false, err
	}
	return token == verify, nil
}
