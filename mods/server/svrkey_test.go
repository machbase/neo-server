package server_test

import (
	"context"
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
		incomming, err := listener.Accept()
		if err != nil {
			t.Log(err.Error())
			t.Error(err)
			return
		}
		t.Log("server accepted")
		conn, ok := incomming.(*tls.Conn)
		require.True(t, ok)

		err = conn.HandshakeContext(context.Background())
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
	laddr := listener.Addr().(*net.TCPAddr)
	hostport := fmt.Sprintf("%s:%d", laddr.IP.String(), laddr.Port)
	t.Log("client dialing...", laddr.Network(), hostport)
	conn, err := tls.Dial(laddr.Network(), hostport, cliTlsConf)
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
