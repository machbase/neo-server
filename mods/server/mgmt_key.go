package server

import (
	"context"
	"crypto"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"

	"github.com/machbase/neo-grpc/mgmt"
	"github.com/pkg/errors"
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
		rsp.Reason = "id contains invalid character"
		return rsp, nil
	}
	if len(req.Id) > 40 {
		rsp.Reason = "id is too long, should be shorter than 40 characters"
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

func (s *svr) ServerKey(ctx context.Context, req *mgmt.ServerKeyRequest) (*mgmt.ServerKeyResponse, error) {
	tick := time.Now()
	rsp := &mgmt.ServerKeyResponse{Reason: "unspecified"}
	defer func() {
		rsp.Elapse = time.Since(tick).String()
	}()
	path := s.ServerCertificatePath()
	b, err := os.ReadFile(path)
	if err != nil {
		rsp.Reason = err.Error()
	} else {
		rsp.Success = true
		rsp.Reason = "success"
		rsp.Certificate = string(b)
	}
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

// generateClientKey returns certificate, privatekey, token and error
func generateClientKey(req *GenCertReq) ([]byte, []byte, string, error) {
	var clientKey any
	var clientPub any
	var clientKeyPEM []byte

	switch req.Type {
	case "rsa":
		bitSize := 4096
		key, err := rsa.GenerateKey(rand.Reader, bitSize)
		if err != nil {
			return nil, nil, "", err
		}
		clientKey = key
		clientPub = &key.PublicKey
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
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	case "ec", "ecdsa":
		ec := NewEllipticCurveP521()
		pri, pub, err := ec.GenerateKeys()
		if err != nil {
			return nil, nil, "", err
		}
		clientKey = pri
		clientPub = pub
		marshal, err := x509.MarshalECPrivateKey(pri)
		if err != nil {
			return nil, nil, "", err
		}
		clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: marshal})
	default:
		return nil, nil, "", errors.New("unsupported key type")
	}

	token, err := GenerateClientToken(req.CommonName, clientKey, "b")
	if err != nil {
		return nil, nil, "", err
	}

	certBytes, err := GenerateClientCertificate(req.Name, req.NotBefore, req.NotAfter, req.Issuer, req.IssuerKey, clientPub)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "client certificate")
	}

	return certBytes, clientKeyPEM, token, nil
}

// func hashCertificate(cert *x509.Certificate) (string, error) {
// 	raw := cert.Raw
// 	b64str := base64.StdEncoding.EncodeToString(raw)
// 	b64str = strings.TrimSpace(b64str)
// 	sha := sha3.New256()
// 	sha.Write([]byte(b64str))
// 	return hex.EncodeToString(sha.Sum(nil)), nil
// }

func GenerateClientToken(clientId string, clientPriKey crypto.PrivateKey, method string) (token string, err error) {
	var signature []byte
	hash := sha256.New()
	hash.Write([]byte(clientId))
	hashsum := hash.Sum(nil)
	switch key := clientPriKey.(type) {
	case *rsa.PrivateKey:
		signature, err = rsa.SignPSS(rand.Reader, key, crypto.SHA256, hashsum, nil)
		if err != nil {
			return "", err
		}
	case *ecdsa.PrivateKey:
		signature, err = ecdsa.SignASN1(rand.Reader, key, hashsum)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported algorithm '%T'", key)
	}
	if method != "b" {
		return "", fmt.Errorf("unsupported method '%s'", method)
	}
	token = fmt.Sprintf("%s:%s:%s", clientId, method, hex.EncodeToString(signature))
	return
}

func VerifyClientToken(token string, clientPubKey crypto.PublicKey) (bool, error) {
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 {
		return false, errors.New("invalid token format")
	}

	if parts[1] != "b" {
		return false, fmt.Errorf("unsupported method '%s'", parts[1])
	}

	signature, err := hex.DecodeString(parts[2])
	if err != nil {
		return false, err
	}

	hash := sha256.New()
	hash.Write([]byte(parts[0]))
	hashsum := hash.Sum(nil)

	switch key := clientPubKey.(type) {
	case *rsa.PublicKey:
		err = rsa.VerifyPSS(key, crypto.SHA256, hashsum, signature, nil)
		if err != nil {
			fmt.Printf("rsa <<< %s", err.Error())
			return false, err
		}
		return err == nil, err
	case *ecdsa.PublicKey:
		return ecdsa.VerifyASN1(key, hashsum, signature), nil
	default:
		return false, fmt.Errorf("unsupproted algorithm '%T'", key)
	}
}
