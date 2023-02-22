package server

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"reflect"
	"time"

	"github.com/pkg/errors"
)

type EllipticCurve struct {
	pubKeyCurve elliptic.Curve
	privateKey  *ecdsa.PrivateKey
	publicKey   *ecdsa.PublicKey
}

func NewEllipticCurveP521() *EllipticCurve {
	return NewEllipticCurve(elliptic.P521())
}

func NewEllipticCurve(curve elliptic.Curve) *EllipticCurve {
	return &EllipticCurve{
		pubKeyCurve: curve,
		privateKey:  new(ecdsa.PrivateKey),
	}
}

func (ec *EllipticCurve) GenerateKeys() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	var err error
	privKey, err := ecdsa.GenerateKey(ec.pubKeyCurve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	ec.privateKey = privKey
	ec.publicKey = &privKey.PublicKey
	return ec.privateKey, ec.publicKey, nil
}

// EncodePrivate private key
func (ec *EllipticCurve) EncodePrivate(privKey *ecdsa.PrivateKey) (string, error) {
	encoded, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return "", err
	}
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encoded})
	return string(pemEncoded), nil
}

// EncodePublic public key
func (ec *EllipticCurve) EncodePublic(pubKey *ecdsa.PublicKey) (string, error) {
	encoded, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", err
	}
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded})
	return string(pemEncodedPub), nil
}

// DecodePrivate private key
func (ec *EllipticCurve) DecodePrivate(pemEncodedPriv string) (*ecdsa.PrivateKey, error) {
	blockPriv, _ := pem.Decode([]byte(pemEncodedPriv))
	x509EncodedPriv := blockPriv.Bytes
	privateKey, err := x509.ParseECPrivateKey(x509EncodedPriv)
	return privateKey, err
}

// DecodePublic public key
func (ec *EllipticCurve) DecodePublic(pemEncodedPub string) (*ecdsa.PublicKey, error) {
	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, err := x509.ParsePKIXPublicKey(x509EncodedPub)
	publicKey := genericPublicKey.(*ecdsa.PublicKey)
	return publicKey, err
}

// VerifySignature sign ecdsa style and verify signature
func (ec *EllipticCurve) VerifySignature(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) ([]byte, bool, error) {
	h := md5.New()
	io.WriteString(h, "This is a message to be signed and verified by ECDSA!")
	signhash := h.Sum(nil)

	r, s, serr := ecdsa.Sign(rand.Reader, privKey, signhash)
	if serr != nil {
		return []byte(""), false, serr
	}

	signature := r.Bytes()
	signature = append(signature, s.Bytes()...)

	verify := ecdsa.Verify(pubKey, signhash, r, s)

	return signature, verify, nil
}

// Test encode, decode and test it with deep equal
func (ec *EllipticCurve) Test(privKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) error {
	encPriv, err := ec.EncodePrivate(privKey)
	if err != nil {
		return err
	}
	encPub, err := ec.EncodePublic(pubKey)
	if err != nil {
		return err
	}
	priv2, err := ec.DecodePrivate(encPriv)
	if err != nil {
		return err
	}
	pub2, err := ec.DecodePublic(encPub)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(privKey, priv2) {
		return errors.New("private keys do not match")
	}
	if !reflect.DeepEqual(pubKey, pub2) {
		return errors.New("public keys do not match")
	}
	return nil
}

func GenerateServerCertificate(priKey *ecdsa.PrivateKey, pubKey *ecdsa.PublicKey) ([]byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject: pkix.Name{
			CommonName:         "machbase-neo",
			OrganizationalUnit: []string{"R&D Center"},
			Organization:       []string{"machbase.com"},
			StreetAddress:      []string{"3003 N First St #206"},
			PostalCode:         []string{"95134"},
			Locality:           []string{"San Jose"},
			Country:            []string{"CA"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, pubKey, priKey)
	if err != nil {
		return nil, err
	}

	out := &bytes.Buffer{}
	err = pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func GenerateClientCertificate(subject pkix.Name, notBefore time.Time, notAfter time.Time, ca *x509.Certificate, caKey crypto.PrivateKey, pubKey crypto.PublicKey) ([]byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		IsCA:                  false,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		Subject:               subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

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
