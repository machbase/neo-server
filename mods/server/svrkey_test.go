package server_test

import (
	"bytes"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	. "github.com/machbase/neo-server/mods/server"
	"github.com/stretchr/testify/require"
)

func TestKeyGen(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	pripem, err := ec.EncodePrivate(pri)
	require.Nil(t, err)
	require.NotEmpty(t, pripem)

	pubpem, err := ec.EncodePublic(pub)
	require.Nil(t, err)
	require.NotEmpty(t, pubpem)

	fmt.Println(pripem)
	fmt.Println(pubpem)
}

func TestCert(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		panic(err)
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
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, pub, pri)
	if err != nil {
		panic(err)
	}

	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	t.Log(out.String())
}
