package sshd

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

func signerFromPath(keypath, password string) (ssh.Signer, error) {
	var signer ssh.Signer
	if len(keypath) > 0 {
		pemBytes, err := os.ReadFile(keypath)
		if err != nil {
			return signer, errors.Wrap(err, "server key")
		}
		var keypass []byte
		if len(password) > 0 {
			keypass = []byte(password)
		}
		signer, err = signerFromPem(pemBytes, keypass)
		if err != nil {
			return signer, errors.Wrap(err, "server signer")
		}
	}
	return signer, nil
}

func signerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {
	// read pem block
	err := errors.New("Pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	if password != nil {
		// decrypt PEM
		// TODO legacy PEM RFC1423 is insecure
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
		if err != nil {
			return nil, fmt.Errorf("decrypting PEM block failed %v", err)
		}
	}

	// get RSA, EC or DSA key
	key, err := parsePemBlock(pemBlock)
	if err != nil {
		return nil, err
	}

	// generate signer instance from key
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, fmt.Errorf("creating signer from encrypted key failed %v", err)
	}

	return signer, nil
}

func parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		//key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("parsing private key failed, unsupported key type %q", block.Type)
	}
}
