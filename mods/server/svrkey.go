package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"reflect"

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
