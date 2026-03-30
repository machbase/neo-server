package server_test

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"testing"
	"time"

	. "github.com/machbase/neo-server/v8/mods/server"
	"github.com/stretchr/testify/require"
)

func TestKeyGen(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, pri)
	require.NotNil(t, pub)

	pri_pem, err := ec.EncodePrivate(pri)
	require.Nil(t, err)
	require.NotEmpty(t, pri_pem)

	pub_pem, err := ec.EncodePublic(pub)
	require.Nil(t, err)
	require.NotEmpty(t, pub_pem)

	fmt.Println(pri_pem)
	fmt.Println(pub_pem)
}

func TestKeyEncodeDecode(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	// Encode and decode private key
	priPem, err := ec.EncodePrivate(pri)
	require.Nil(t, err)
	decodedPri, err := ec.DecodePrivate(priPem)
	require.Nil(t, err)
	require.True(t, pri.Equal(decodedPri))

	// Encode and decode public key
	pubPem, err := ec.EncodePublic(pub)
	require.Nil(t, err)
	decodedPub, err := ec.DecodePublic(pubPem)
	require.Nil(t, err)
	require.True(t, pub.Equal(decodedPub))
}

func TestKeyVerifySignature(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	sig, verified, err := ec.VerifySignature(pri, pub)
	require.Nil(t, err)
	require.True(t, verified)
	require.NotEmpty(t, sig)
}

func TestKeyTest(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	err = ec.Test(pri, pub)
	require.Nil(t, err)
}

func TestHashCertificate(t *testing.T) {
	ec := NewEllipticCurveP521()
	pri, pub, err := ec.GenerateKeys()
	require.Nil(t, err)

	certPEM, err := GenerateServerCertificate(pri, pub)
	require.Nil(t, err)

	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.Nil(t, err)

	hash, err := HashCertificate(cert)
	require.Nil(t, err)
	require.NotEmpty(t, hash)
	// SHA3-256 produces 64 hex chars
	require.Len(t, hash, 64)
}

func TestCert(t *testing.T) {
	ec := NewEllipticCurveP521()

	// server key
	svrPri, svrPub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, svrPri)
	require.NotNil(t, svrPub)

	svrKeyPEMStr, err := ec.EncodePrivate(svrPri)
	svrKeyPEM := []byte(svrKeyPEMStr)
	require.Nil(t, err)
	require.True(t, len(svrKeyPEM) > 0)

	// server cert
	svrCertPEM, err := GenerateServerCertificate(svrPri, svrPub)
	require.Nil(t, err)
	require.NotNil(t, svrCertPEM)

	// re-parse server cert
	block, _ := pem.Decode([]byte(svrCertPEM))
	svrCert, err := x509.ParseCertificate(block.Bytes)
	require.Nil(t, err)
	require.NotNil(t, svrCert)

	// client key
	ec = NewEllipticCurveP521()
	cliPri, cliPub, err := ec.GenerateKeys()
	require.Nil(t, err)
	require.NotNil(t, cliPri)
	require.NotNil(t, cliPub)

	cliKeyPEMStr, err := ec.EncodePrivate(cliPri)
	cliKeyPEM := []byte(cliKeyPEMStr)
	require.Nil(t, err)
	require.True(t, len(cliKeyPEM) > 0)

	// client cert
	cliCertPEM, err := GenerateClientCertificate(pkix.Name{CommonName: "TheClient"}, time.Now(), time.Now().Add(time.Hour), svrCert, svrPri, cliPub)
	require.Nil(t, err)
	require.NotNil(t, cliCertPEM)

	// configure server TLS
	var rootCAs *x509.CertPool = x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(svrCertPEM)

	svrKeyPair, err := tls.X509KeyPair(svrCertPEM, svrKeyPEM)
	if err != nil {
		t.Error(err)
		return
	}
	svrTlsConf := &tls.Config{
		Certificates: []tls.Certificate{svrKeyPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootCAs,
	}

	// test SSL
	listener := newLocalListener(t, svrTlsConf)
	defer listener.Close()

	complete := make(chan bool)
	defer close(complete)

	go func() {
		t.Log("server listening...", listener.Addr().String())
		incoming, err := listener.Accept()
		if err != nil {
			t.Log(err.Error())
			t.Error(err)
			return
		}
		t.Log("server accepted")
		conn, ok := incoming.(*tls.Conn)
		require.True(t, ok)

		err = conn.HandshakeContext(t.Context())
		require.Nil(t, err)
		t.Log("server-client handshake done")

		<-complete
		conn.Close()
	}()

	// configure client TLS
	cliKeyPair, err := tls.X509KeyPair(cliCertPEM, cliKeyPEM)
	if err != nil {
		panic(err)
	}
	cliTlsConf := &tls.Config{
		Certificates:       []tls.Certificate{cliKeyPair},
		RootCAs:            rootCAs,
		InsecureSkipVerify: true,
	}
	listener_addr := listener.Addr().(*net.TCPAddr)
	hostPort := fmt.Sprintf("%s:%d", listener_addr.IP.String(), listener_addr.Port)
	t.Log("client dialing...", listener_addr.Network(), hostPort)
	conn, err := tls.Dial(listener_addr.Network(), hostPort, cliTlsConf)
	require.Nil(t, err)
	require.NotNil(t, conn)
	conn.Close()
	t.Log("client done")
	complete <- true
}

func newLocalListener(t *testing.T, tlsConf *tls.Config) net.Listener {
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsConf)
	if err != nil {
		ln, err = tls.Listen("tcp6", "[::1]:0", tlsConf)
	}
	if err != nil {
		t.Fatal(err)
	}
	return ln
}
